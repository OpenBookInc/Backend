use tonic::{transport::Server, Request, Response, Status};
use std::sync::{Arc, Mutex};
use tokio_stream::StreamExt;
use tokio_stream::wrappers::ReceiverStream;
use std::pin::Pin;
use futures::Stream;

use matching_server::matching_service_package::matching_server_service_server::{MatchingServerService, MatchingServerServiceServer};
use matching_server::matching_service_package::{
    GatewayMessage, EngineMessage,
    gateway_message::Msg,
    engine_message::Event,
    SequencedMessageBase, ResponseBase, FallibleBase,
    NewOrderAcknowledgement, CancelOrderAcknowledgement, OrderElimination, Match,
};

use matching_server::pool_manager::PoolManager;

/// Inner state of the MatcherService, wrapped in Arc to allow cloning into spawned tasks.
#[derive(Clone)]
struct ServiceState {
    /// Tracks the next expected sequence number from incoming messages.
    expected_next_sequence_number: Arc<Mutex<u64>>,

    /// The pool manager that handles order matching logic.
    pool_manager: Arc<Mutex<PoolManager>>,

    /// Tracks the next response sequence number for outgoing messages.
    next_response_sequence_number: Arc<Mutex<u64>>,

    /// Response channel for the trade stream.
    trade_tx: Arc<Mutex<Option<tokio::sync::mpsc::Sender<Result<EngineMessage, Status>>>>>,
}

pub struct MatcherService {
    state: ServiceState,
}

impl MatcherService {
    pub fn new() -> Self {
        Self {
            state: ServiceState {
                expected_next_sequence_number: Arc::new(Mutex::new(0)),
                pool_manager: Arc::new(Mutex::new(PoolManager::new())),
                next_response_sequence_number: Arc::new(Mutex::new(0)),
                trade_tx: Arc::new(Mutex::new(None)),
            },
        }
    }

    /// Closes the trade stream and resets all state for a fresh reconnection.
    fn close_stream(&self) {
        let _ = self.state.trade_tx.lock().unwrap().take();
        *self.state.expected_next_sequence_number.lock().unwrap() = 0;
        *self.state.next_response_sequence_number.lock().unwrap() = 0;
        *self.state.pool_manager.lock().unwrap() = PoolManager::new();
    }

    /// Helper to get the next response sequence number
    fn get_next_response_sequence(&self) -> u64 {
        let mut seq = self.state.next_response_sequence_number.lock().unwrap();
        let current = *seq;
        *seq += 1;
        current
    }

    /// Validates the incoming sequence number and updates expected.
    /// Returns an error if sequence number doesn't match expected.
    fn validate_and_advance_sequence(&self, received_sequence: u64) -> Result<(), Status> {
        let mut expected = self.state.expected_next_sequence_number.lock().unwrap();
        if received_sequence != *expected {
            return Err(Status::invalid_argument(format!(
                "Sequence number mismatch: expected {}, received {}",
                *expected, received_sequence
            )));
        }
        *expected += 1;
        Ok(())
    }

    /// Processes a GatewayMessage and sends responses through the trade channel.
    async fn process_gateway_message(
        &self,
        message: GatewayMessage,
        tx: &tokio::sync::mpsc::Sender<Result<EngineMessage, Status>>,
    ) -> Result<(), Status> {
        // Extract and validate sequence number
        let sequenced_base = message
            .sequenced_message_base
            .as_ref()
            .ok_or_else(|| Status::invalid_argument("Missing sequenced_message_base"))?;

        let request_sequence_number = sequenced_base.sequence_number;
        self.validate_and_advance_sequence(request_sequence_number)?;

        // Process based on message type
        match message.msg {
            Some(Msg::NewOrder(new_order)) => {
                println!("Processing NewOrder: {:?}", new_order);

                // Extract body
                let body = new_order
                    .body
                    .ok_or_else(|| Status::invalid_argument("Missing order body"))?;

                // Process the order through the pool manager
                let result = {
                    let mut pool_manager = self.state.pool_manager.lock().unwrap();
                    pool_manager.create_entry(body)
                };

                match result {
                    Ok((elimination_bodies, ack_body, match_bodies, market_entry_elimination_bodies)) => {
                        // Send OrderElimination messages first (prior to acknowledgement)
                        for elimination_body in elimination_bodies {
                            let elimination_message = EngineMessage {
                                sequenced_message_base: Some(SequencedMessageBase {
                                    sequence_number: self.get_next_response_sequence(),
                                }),
                                event: Some(Event::Elimination(OrderElimination {
                                    body: Some(elimination_body),
                                })),
                            };

                            tx.send(Ok(elimination_message)).await.map_err(|_| {
                                Status::internal("Failed to send OrderElimination")
                            })?;
                        }

                        // Send NewOrderAcknowledgement (success)
                        let ack_message = EngineMessage {
                            sequenced_message_base: Some(SequencedMessageBase {
                                sequence_number: self.get_next_response_sequence(),
                            }),
                            event: Some(Event::NewOrderAcknowledgement(NewOrderAcknowledgement {
                                response_base: Some(ResponseBase {
                                    request_sequence_number,
                                }),
                                fallible_base: Some(FallibleBase {
                                    success: true,
                                    error_description: String::new(),
                                }),
                                body: Some(ack_body),
                            })),
                        };

                        tx.send(Ok(ack_message)).await.map_err(|_| {
                            Status::internal("Failed to send NewOrderAcknowledgement")
                        })?;

                        // Send Match messages
                        for match_body in match_bodies {
                            let match_message = EngineMessage {
                                sequenced_message_base: Some(SequencedMessageBase {
                                    sequence_number: self.get_next_response_sequence(),
                                }),
                                event: Some(Event::Match(Match {
                                    body: Some(match_body),
                                })),
                            };

                            tx.send(Ok(match_message)).await.map_err(|_| {
                                Status::internal("Failed to send Match")
                            })?;
                        }

                        // Send market entry OrderElimination messages (after matches)
                        for elimination_body in market_entry_elimination_bodies {
                            let elimination_message = EngineMessage {
                                sequenced_message_base: Some(SequencedMessageBase {
                                    sequence_number: self.get_next_response_sequence(),
                                }),
                                event: Some(Event::Elimination(OrderElimination {
                                    body: Some(elimination_body),
                                })),
                            };

                            tx.send(Ok(elimination_message)).await.map_err(|_| {
                                Status::internal("Failed to send market entry OrderElimination")
                            })?;
                        }
                    }
                    Err(error_msg) => {
                        // Send NewOrderAcknowledgement (failure)
                        let ack_message = EngineMessage {
                            sequenced_message_base: Some(SequencedMessageBase {
                                sequence_number: self.get_next_response_sequence(),
                            }),
                            event: Some(Event::NewOrderAcknowledgement(NewOrderAcknowledgement {
                                response_base: Some(ResponseBase {
                                    request_sequence_number,
                                }),
                                fallible_base: Some(FallibleBase {
                                    success: false,
                                    error_description: error_msg,
                                }),
                                body: None,
                            })),
                        };

                        tx.send(Ok(ack_message)).await.map_err(|_| {
                            Status::internal("Failed to send NewOrderAcknowledgement")
                        })?;
                    }
                }
            }
            Some(Msg::CancelOrder(cancel_order)) => {
                println!("Processing CancelOrder: {:?}", cancel_order);

                // Extract body
                let body = cancel_order
                    .body
                    .ok_or_else(|| Status::invalid_argument("Missing cancel body"))?;

                // Process the cancel through the pool manager
                let result = {
                    let mut pool_manager = self.state.pool_manager.lock().unwrap();
                    pool_manager.cancel_entry(body)
                };

                match result {
                    Ok(ack_body) => {
                        // Send CancelOrderAcknowledgement (success)
                        let ack_message = EngineMessage {
                            sequenced_message_base: Some(SequencedMessageBase {
                                sequence_number: self.get_next_response_sequence(),
                            }),
                            event: Some(Event::CancelOrderAcknowledgement(CancelOrderAcknowledgement {
                                response_base: Some(ResponseBase {
                                    request_sequence_number,
                                }),
                                fallible_base: Some(FallibleBase {
                                    success: true,
                                    error_description: String::new(),
                                }),
                                body: Some(ack_body),
                            })),
                        };

                        tx.send(Ok(ack_message)).await.map_err(|_| {
                            Status::internal("Failed to send CancelOrderAcknowledgement")
                        })?;
                    }
                    Err(error_msg) => {
                        // Send CancelOrderAcknowledgement (failure)
                        let ack_message = EngineMessage {
                            sequenced_message_base: Some(SequencedMessageBase {
                                sequence_number: self.get_next_response_sequence(),
                            }),
                            event: Some(Event::CancelOrderAcknowledgement(CancelOrderAcknowledgement {
                                response_base: Some(ResponseBase {
                                    request_sequence_number,
                                }),
                                fallible_base: Some(FallibleBase {
                                    success: false,
                                    error_description: error_msg,
                                }),
                                body: None,
                            })),
                        };

                        tx.send(Ok(ack_message)).await.map_err(|_| {
                            Status::internal("Failed to send CancelOrderAcknowledgement")
                        })?;
                    }
                }
            }
            None => {
                return Err(Status::invalid_argument("GatewayMessage has no msg set"));
            }
        }

        Ok(())
    }
}

#[tonic::async_trait]
impl MatchingServerService for MatcherService {
    type CreateTradeStreamStream = Pin<Box<dyn Stream<Item = Result<EngineMessage, Status>> + Send>>;

    /// Handles the single bidirectional trade stream.
    /// This handler is called ONCE when the client establishes the connection.
    /// The returned stream persists for the lifetime of the client session.
    async fn create_trade_stream(
        &self,
        request: Request<tonic::Streaming<GatewayMessage>>,
    ) -> Result<Response<Self::CreateTradeStreamStream>, Status> {
        let mut in_stream = request.into_inner();

        // Create a channel for sending responses back to the client
        let (tx, rx) = tokio::sync::mpsc::channel(100);

        // Store the sender so we can close the stream if needed
        *self.state.trade_tx.lock().unwrap() = Some(tx.clone());

        // Clone the state for the spawned task
        let state = self.state.clone();

        // Spawn a task to process incoming messages
        tokio::spawn(async move {
            let service = MatcherService { state };

            while let Some(result) = in_stream.next().await {
                match result {
                    Ok(message) => {
                        if let Err(e) = service.process_gateway_message(message, &tx).await {
                            eprintln!("ERROR: Trade stream error: {:?}. Closing stream.", e);
                            service.close_stream();
                            break;
                        }
                    }
                    Err(_) => {
                        // Stream error (client disconnect) - close stream
                        service.close_stream();
                        break;
                    }
                }
            }
            // Stream ended normally (client closed connection)
            service.close_stream();
            println!("Trade stream closed");
        });

        println!("Trade stream created");

        // Create the output stream from the receiver
        let out_stream = ReceiverStream::new(rx);
        Ok(Response::new(Box::pin(out_stream) as Self::CreateTradeStreamStream))
    }
}

/// Configuration loaded from .env file
struct Config {
    engine_port: u16,
}

impl Config {
    fn load() -> Result<Self, Box<dyn std::error::Error>> {
        // Load .env file from current directory
        dotenvy::dotenv().map_err(|e| {
            format!("Failed to load .env file: {}. Please ensure .env file exists with required fields (ENGINE_PORT)", e)
        })?;

        // Read required ENGINE_PORT
        let engine_port = std::env::var("ENGINE_PORT")
            .map_err(|_| "Missing required environment variable: ENGINE_PORT")?
            .parse::<u16>()
            .map_err(|_| "ENGINE_PORT must be a valid port number (0-65535)")?;

        Ok(Config {
            engine_port,
        })
    }
}

#[tokio::main]
async fn main() -> Result<(), Box<dyn std::error::Error>> {
    // Load configuration from .env file
    let config = Config::load()?;

    // Construct server address using configured port
    let addr = format!("[::1]:{}", config.engine_port).parse()?;
    let matcher = MatcherService::new();

    println!("Matcher gRPC server listening on {}", addr);

    Server::builder()
        .add_service(MatchingServerServiceServer::new(matcher))
        .serve(addr)
        .await?;

    Ok(())
}
