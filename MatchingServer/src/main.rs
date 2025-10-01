use tonic::{transport::Server, Request, Response, Status};

pub mod service {
    tonic::include_proto!("service");
}

use service::command_center_matcher_server::{CommandCenterMatcher, CommandCenterMatcherServer};
use service::{LogEntry, LogResponse};

#[derive(Debug, Default)]
pub struct MatcherService {}

#[tonic::async_trait]
impl CommandCenterMatcher for MatcherService {
    async fn send_log(
        &self,
        request: Request<LogEntry>,
    ) -> Result<Response<LogResponse>, Status> {
        let log = request.into_inner();
        println!("Received log: {:?}", log.content);

        let response = LogResponse {
            matched: true,
            pattern: "example".to_string(),
            message: "Log processed".to_string(),
        };

        Ok(Response::new(response))
    }

    async fn stream_logs(
        &self,
        request: Request<tonic::Streaming<LogEntry>>,
    ) -> Result<Response<tonic::codec::Streaming<LogResponse>>, Status> {
        unimplemented!("Streaming not yet implemented")
    }
}

#[tokio::main]
async fn main() -> Result<(), Box<dyn std::error::Error>> {
    let addr = "[::1]:50051".parse()?;
    let matcher = MatcherService::default();

    println!("Matcher gRPC server listening on {}", addr);

    Server::builder()
        .add_service(CommandCenterMatcherServer::new(matcher))
        .serve(addr)
        .await?;

    Ok(())
}
