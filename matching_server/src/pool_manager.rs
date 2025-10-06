// pool_manager.rs
//
// PoolManager coordinates multiple EntryPools and handles the translation between
// protobuf messages and the EntryPool API.

use std::collections::HashMap;

// Import the EntryPool types
use crate::entry_pool::{EntryPool, EntryParameters, EntryType};

// Import protobuf generated types
use crate::matching_service_package::{
    order_new, pool_definition_request, pool_definition_response, fill_event, OrderType
};

/// FallibleBase for error handling (not in the main proto but composed in responses)
#[derive(Debug, Clone)]
pub struct FallibleBase {
    pub success: bool,
    pub error_description: String,
}

impl FallibleBase {
    pub fn error(description: String) -> Self {
        FallibleBase {
            success: false,
            error_description: description,
        }
    }
}

/// Manages multiple entry pools and coordinates order routing
pub struct PoolManager {
    /// Map from pool_id to EntryPool
    pools: HashMap<u64, PoolInfo>,
    /// Counter for generating unique order IDs across all pools
    next_order_id: u64,
    /// Counter for generating unique transaction IDs
    next_transaction_id: u64,
    /// Counter for generating unique fill event IDs
    next_fill_event_id: u64,
    /// Counter for generating unique fill IDs
    next_fill_id: u64,
}

/// Information about a pool
struct PoolInfo {
    pool: EntryPool,
    leg_security_ids: Vec<u64>,
}

impl PoolManager {
    /// Creates a new PoolManager
    pub fn new() -> Self {
        PoolManager {
            pools: HashMap::new(),
            next_order_id: 1,
            next_transaction_id: 1,
            next_fill_event_id: 1,
            next_fill_id: 1,
        }
    }

    /// Defines a new pool and returns the pool definition response
    pub fn define_pool(
        &mut self,
        request: pool_definition_request::Body,
    ) -> Result<pool_definition_response::Body, FallibleBase> {
        // Validate pool doesn't already exist
        if self.pools.contains_key(&request.pool_id) {
            return Err(FallibleBase::error(format!(
                "Pool {} already exists",
                request.pool_id
            )));
        }

        // Validate we have at least one leg
        if request.leg_security_ids.is_empty() {
            return Err(FallibleBase::error(
                "Pool must have at least one leg".to_string(),
            ));
        }

        let num_legs = request.leg_security_ids.len();

        // Create the entry pool
        let pool = EntryPool::new(request.total_units, num_legs);

        // Generate all lineups (2^N combinations)
        let num_lineups = 1 << num_legs;
        let mut lineups = Vec::with_capacity(num_lineups);

        for lineup_index in 0..num_lineups {
            let mut legs = Vec::with_capacity(num_legs);

            // Use binary representation: bit i indicates over (1) or under (0) for leg i
            for leg_idx in 0..num_legs {
                let is_over = (lineup_index & (1 << leg_idx)) != 0;
                legs.push(pool_definition_response::body::lineup::Leg {
                    security_id: request.leg_security_ids[leg_idx],
                    is_over,
                });
            }

            lineups.push(pool_definition_response::body::Lineup {
                lineup_index: lineup_index as u64,
                legs,
            });
        }

        // Store the pool
        self.pools.insert(
            request.pool_id,
            PoolInfo {
                pool,
                leg_security_ids: request.leg_security_ids.clone(),
            },
        );

        Ok(pool_definition_response::Body {
            pool_id: request.pool_id,
            lineups,
        })
    }

    /// Creates a new entry/order and returns acknowledgement and any fill events
    pub fn create_entry(
        &mut self,
        order: order_new::Body,
    ) -> Result<(crate::matching_service_package::order_new_acknowledgement::Body, Vec<fill_event::Body>), FallibleBase> {
        // Validate pool exists
        let pool_info = self.pools.get_mut(&order.pool_id).ok_or_else(|| {
            FallibleBase::error(format!("Pool {} does not exist", order.pool_id))
        })?;

        // Convert OrderType to EntryType
        let entry_type = match OrderType::try_from(order.order_type) {
            Ok(OrderType::Limit) => EntryType::Limit,
            Ok(OrderType::Market) => EntryType::Market,
            Err(_) => return Err(FallibleBase::error("Invalid order type".to_string())),
        };

        // Get the next order id
        let order_id = self.next_order_id;
        self.next_order_id += 1;

        // Create entry parameters
        let params = EntryParameters {
            entry_id: order_id,
            entry_type,
            portion: order.portion,
            quantity: order.quantity,
        };

        // Submit to the entry pool
        let submit_result = pool_info
            .pool
            .submit_entry(order.lineup_index as usize, params)
            .map_err(|e| FallibleBase::error(e))?;

        // Create acknowledgement
        let ack = crate::matching_service_package::order_new_acknowledgement::Body { 
            order_id 
        };

        // Convert fill events
        let transaction_id = if !submit_result.fill_events.is_empty() {
            let tid = self.next_transaction_id;
            self.next_transaction_id += 1;
            tid
        } else {
            0 // Not used if no fills
        };

        let mut fill_event_bodies = Vec::new();
        for fill_event in submit_result.fill_events {
            let fill_event_id = self.next_fill_event_id;
            self.next_fill_event_id += 1;

            let mut fills = Vec::new();
            for filled_entry in fill_event.filled_entries {
                let fill_id = self.next_fill_id;
                self.next_fill_id += 1;

                fills.push(fill_event::body::Fill {
                    fill_id,
                    order_id: filled_entry.entry.id,
                    is_aggressor: filled_entry.entry.lineup_index
                        == fill_event.aggressor_lineup_index,
                    matched_portion: filled_entry.matched_portion,
                });
            }

            fill_event_bodies.push(fill_event::Body {
                transaction_id,
                fill_event_id,
                matched_quantity: fill_event.matched_quantity,
                fills,
            });
        }

        Ok((ack, fill_event_bodies))
    }

    /// Returns the number of pools currently managed
    pub fn num_pools(&self) -> usize {
        self.pools.len()
    }

    /// Gets a reference to a specific pool's EntryPool (for testing/debugging)
    pub fn get_pool(&self, pool_id: u64) -> Option<&EntryPool> {
        self.pools.get(&pool_id).map(|info| &info.pool)
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_define_pool() {
        let mut manager = PoolManager::new();

        let request = pool_definition_request::Body {
            pool_id: 1,
            total_units: 1000,
            leg_security_ids: vec![101, 102],
        };

        let response = manager.define_pool(request).unwrap();

        assert_eq!(response.pool_id, 1);
        assert_eq!(response.lineups.len(), 4); // 2^2 = 4 lineups

        // Verify lineup 0: both under (binary 00)
        assert_eq!(response.lineups[0].lineup_index, 0);
        assert_eq!(response.lineups[0].legs[0].security_id, 101);
        assert_eq!(response.lineups[0].legs[0].is_over, false);
        assert_eq!(response.lineups[0].legs[1].security_id, 102);
        assert_eq!(response.lineups[0].legs[1].is_over, false);

        // Verify lineup 3: both over (binary 11)
        assert_eq!(response.lineups[3].lineup_index, 3);
        assert_eq!(response.lineups[3].legs[0].is_over, true);
        assert_eq!(response.lineups[3].legs[1].is_over, true);
    }

    #[test]
    fn test_duplicate_pool_definition() {
        let mut manager = PoolManager::new();

        let request = pool_definition_request::Body {
            pool_id: 1,
            total_units: 1000,
            leg_security_ids: vec![101, 102],
        };

        manager.define_pool(request.clone()).unwrap();
        let result = manager.define_pool(request);

        assert!(result.is_err());
    }

    #[test]
    fn test_create_entry_with_fill() {
        let mut manager = PoolManager::new();

        // Define pool
        manager
            .define_pool(pool_definition_request::Body {
                pool_id: 1,
                total_units: 1000,
                leg_security_ids: vec![101, 102],
            })
            .unwrap();

        // Submit entries to all 4 lineups
        let (ack0, fills0) = manager
            .create_entry(order_new::Body {
                pool_id: 1,
                lineup_index: 0,
                order_type: OrderType::Limit as i32,
                portion: 250,
                quantity: 250,
                self_match_id: None,
            })
            .unwrap();
        assert_eq!(fills0.len(), 0);

        let (ack1, fills1) = manager
            .create_entry(order_new::Body {
                pool_id: 1,
                lineup_index: 1,
                order_type: OrderType::Limit as i32,
                portion: 250,
                quantity: 250,
                self_match_id: None,
            })
            .unwrap();
        assert_eq!(fills1.len(), 0);

        let (ack2, fills2) = manager
            .create_entry(order_new::Body {
                pool_id: 1,
                lineup_index: 2,
                order_type: OrderType::Limit as i32,
                portion: 250,
                quantity: 250,
                self_match_id: None,
            })
            .unwrap();
        assert_eq!(fills2.len(), 0);

        // Fourth entry should trigger fill
        let (ack3, fills3) = manager
            .create_entry(order_new::Body {
                pool_id: 1,
                lineup_index: 3,
                order_type: OrderType::Limit as i32,
                portion: 250,
                quantity: 250,
                self_match_id: None,
            })
            .unwrap();

        assert_eq!(fills3.len(), 1);
        assert_eq!(fills3[0].fills.len(), 4); // One fill per lineup

        // Verify transaction ID is shared
        let transaction_id = fills3[0].transaction_id;
        assert!(transaction_id > 0);

        // Verify each fill has unique fill_id
        let fill_ids: Vec<u64> = fills3[0].fills.iter().map(|f| f.fill_id).collect();
        assert_eq!(fill_ids.len(), 4);
        for i in 0..4 {
            for j in (i + 1)..4 {
                assert_ne!(fill_ids[i], fill_ids[j]);
            }
        }

        // Verify aggressor is marked
        let aggressor_count = fills3[0].fills.iter().filter(|f| f.is_aggressor).count();
        assert_eq!(aggressor_count, 1);
    }

    #[test]
    fn test_multiple_fill_events_share_transaction_id() {
        let mut manager = PoolManager::new();

        manager
            .define_pool(pool_definition_request::Body {
                pool_id: 1,
                total_units: 1000,
                leg_security_ids: vec![101, 102],
            })
            .unwrap();

        // Submit passive entries with enough quantity for 2 fills
        manager
            .create_entry(order_new::Body {
                pool_id: 1,
                lineup_index: 0,
                order_type: OrderType::Limit as i32,
                portion: 250,
                quantity: 500,
                self_match_id: None,
            })
            .unwrap();

        manager
            .create_entry(order_new::Body {
                pool_id: 1,
                lineup_index: 1,
                order_type: OrderType::Limit as i32,
                portion: 250,
                quantity: 250,
                self_match_id: None,
            })
            .unwrap();

        manager
            .create_entry(order_new::Body {
                pool_id: 1,
                lineup_index: 1,
                order_type: OrderType::Limit as i32,
                portion: 250,
                quantity: 250,
                self_match_id: None,
            })
            .unwrap();

        manager
            .create_entry(order_new::Body {
                pool_id: 1,
                lineup_index: 2,
                order_type: OrderType::Limit as i32,
                portion: 250,
                quantity: 500,
                self_match_id: None,
            })
            .unwrap();

        // Aggressor with enough for 2 fills
        let (_ack, fills) = manager
            .create_entry(order_new::Body {
                pool_id: 1,
                lineup_index: 3,
                order_type: OrderType::Limit as i32,
                portion: 250,
                quantity: 500,
                self_match_id: None,
            })
            .unwrap();

        assert_eq!(fills.len(), 2);

        // Both fill events should share the same transaction_id
        assert_eq!(fills[0].transaction_id, fills[1].transaction_id);

        // But have different fill_event_ids
        assert_ne!(fills[0].fill_event_id, fills[1].fill_event_id);
    }

    #[test]
    fn test_order_to_nonexistent_pool() {
        let mut manager = PoolManager::new();

        let result = manager.create_entry(order_new::Body {
            pool_id: 999,
            lineup_index: 0,
            order_type: OrderType::Limit as i32,
            portion: 250,
            quantity: 250,
            self_match_id: None,
        });

        assert!(result.is_err());
    }

    #[test]
    fn test_market_order() {
        let mut manager = PoolManager::new();

        manager
            .define_pool(pool_definition_request::Body {
                pool_id: 1,
                total_units: 1000,
                leg_security_ids: vec![101],
            })
            .unwrap();

        // Submit passive entry for lineup 0
        manager
            .create_entry(order_new::Body {
                pool_id: 1,
                lineup_index: 0,
                order_type: OrderType::Limit as i32,
                portion: 600,
                quantity: 600,
                self_match_id: None,
            })
            .unwrap();

        // Market order for lineup 1 should calculate portion = 400
        let (_ack, fills) = manager
            .create_entry(order_new::Body {
                pool_id: 1,
                lineup_index: 1,
                order_type: OrderType::Market as i32,
                portion: 0, // Ignored for market orders
                quantity: 400,
                self_match_id: None,
            })
            .unwrap();

        assert_eq!(fills.len(), 1);
        assert_eq!(fills[0].fills.len(), 2);

        // Find the market order fill
        let market_fill = fills[0]
            .fills
            .iter()
            .find(|f| f.is_aggressor)
            .unwrap();
        assert_eq!(market_fill.matched_portion, 400);
    }
}
