// pool_manager.rs
//
// PoolManager coordinates multiple EntryPools and handles the translation between
// protobuf messages and the EntryPool API.

use std::collections::HashMap;

// Import the EntryPool types
use crate::entry_pool::{EntryPool, EntryParameters, EntryType};

// Import pool utilities
use crate::pool_utils::{create_pool_key, calculate_lineup_index, Leg};

// Import protobuf generated types
use crate::matching_service_package::{
    order_new::Body as OrderNewBody,
    order_new::body::Leg as OrderNewLeg,
    order_new_acknowledgement::Body as OrderNewAcknowledgementBody,
    order_cancel::Body as OrderCancelBody,
    order_cancel_acknowledgement::Body as OrderCancelAcknowledgementBody,
    order_elimination::Body as OrderEliminationBody,
    fill_event::Body as FillEventBody,
    fill_event::body::Fill as FillEventBody_Fill,
    OrderType
};

const TOTAL_UNITS: u64 = 1_000_000;

/// Manages multiple entry pools and coordinates order routing
pub struct PoolManager {
    /// Map from sorted leg_security_ids to EntryPool
    pools: HashMap<Vec<u64>, PoolInfo>,
    /// Map from order_id to pool key for order cancellation
    order_to_pool: HashMap<u64, Vec<u64>>,
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
    leg_security_ids: Vec<u64>, // Sorted
}

impl PoolManager {
    /// Creates a new PoolManager
    pub fn new() -> Self {
        PoolManager {
            pools: HashMap::new(),
            order_to_pool: HashMap::new(),
            next_order_id: 1,
            next_transaction_id: 1,
            next_fill_event_id: 1,
            next_fill_id: 1,
        }
    }

    /// Creates a new entry/order and returns eliminations, acknowledgement, and any fill events
    /// Automatically creates the pool if it doesn't exist
    pub fn create_entry(
        &mut self,
        order: OrderNewBody,
    ) -> Result<(Vec<OrderEliminationBody>, OrderNewAcknowledgementBody, Vec<FillEventBody>), String> {
        // Validate we have at least one leg
        if order.legs.is_empty() {
            return Err("Order must have at least one leg".to_string());
        }

        // Extract leg security IDs and convert to internal Leg format
        let leg_security_ids: Vec<u64> = order.legs.iter().map(|l| l.leg_security_id).collect();
        let pool_key = create_pool_key(&leg_security_ids);

        // Convert protobuf legs to internal Leg format
        let legs: Vec<Leg> = order.legs.iter().map(|l| Leg {
            leg_security_id: l.leg_security_id,
            is_over: l.is_over,
        }).collect();

        // Calculate lineup index
        let lineup_index = calculate_lineup_index(&legs);

        // Get or create pool
        if !self.pools.contains_key(&pool_key) {
            let num_legs = pool_key.len();
            let pool = EntryPool::new(TOTAL_UNITS, num_legs);
            self.pools.insert(
                pool_key.clone(),
                PoolInfo {
                    pool,
                    leg_security_ids: pool_key.clone(),
                },
            );
        }

        let pool_info = self.pools.get_mut(&pool_key).unwrap(); // Safe because we just ensured it exists

        // Convert OrderType to EntryType
        let entry_type = match OrderType::try_from(order.order_type) {
            Ok(OrderType::Limit) => EntryType::Limit,
            Ok(OrderType::Market) => EntryType::Market,
            Err(_) => return Err("Invalid order type".to_string()),
        };

        // Get the next order id
        let order_id = self.next_order_id;
        self.next_order_id += 1;

        // Track order to pool mapping for cancellation
        self.order_to_pool.insert(order_id, pool_key.clone());

        // Create entry parameters
        let params = EntryParameters {
            entry_id: order_id,
            entry_type,
            portion: order.portion,
            quantity: order.quantity,
            self_match_id: order.self_match_id,
        };

        // Submit to the entry pool
        let submit_result = pool_info
            .pool
            .submit_entry(lineup_index as usize, params)
            .map_err(|e| e)?;

        // Create elimination bodies for any cancelled entries
        let elimination_bodies: Vec<OrderEliminationBody> = submit_result
            .cancelled_entry_ids
            .iter()
            .map(|&cancelled_id| OrderEliminationBody {
                order_id: cancelled_id,
                elimination_description: "Eliminated due to self-match prevention".to_string(),
            })
            .collect();

        // Create acknowledgement
        let ack = OrderNewAcknowledgementBody {
            client_order_id: order.client_order_id,
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

                fills.push(FillEventBody_Fill {
                    fill_id,
                    order_id: filled_entry.entry.id,
                    is_aggressor: filled_entry.entry.lineup_index
                        == fill_event.aggressor_lineup_index,
                    matched_portion: filled_entry.matched_portion,
                    is_complete: filled_entry.is_complete,
                });
            }

            fill_event_bodies.push(FillEventBody {
                transaction_id,
                fill_event_id,
                matched_quantity: fill_event.matched_quantity,
                fills,
            });
        }

        Ok((elimination_bodies, ack, fill_event_bodies))
    }

    /// Returns the number of pools currently managed
    pub fn num_pools(&self) -> usize {
        self.pools.len()
    }

    /// Gets a reference to a specific pool's EntryPool by leg security IDs (for testing/debugging)
    pub fn get_pool(&self, leg_security_ids: &[u64]) -> Option<&EntryPool> {
        let pool_key = create_pool_key(leg_security_ids);
        self.pools.get(&pool_key).map(|info| &info.pool)
    }

    /// Gets the pool key for a specific order ID (for testing/debugging)
    pub fn get_pool_key_for_order(&self, order_id: u64) -> Option<&Vec<u64>> {
        self.order_to_pool.get(&order_id)
    }

    /// Cancels an existing order
    ///
    /// # Arguments
    /// * `cancel` - The order cancel request containing the order_id
    ///
    /// # Returns
    /// * `Ok(OrderCancelAcknowledgementBody)` - Successful cancellation
    /// * `Err(String)` - Error message if cancellation failed
    pub fn cancel_entry(
        &mut self,
        cancel: OrderCancelBody,
    ) -> Result<OrderCancelAcknowledgementBody, String> {
        let order_id = cancel.order_id;

        // Find which pool this order belongs to
        let pool_key = self
            .order_to_pool
            .get(&order_id)
            .ok_or_else(|| format!("Order ID {} not found", order_id))?
            .clone();

        // Get the pool
        let pool_info = self
            .pools
            .get_mut(&pool_key)
            .ok_or_else(|| format!("Pool not found for order ID {}", order_id))?;

        // Cancel the entry in the pool and return any error result if one occurs
        pool_info.pool.cancel_entry(order_id)?;

        // Remove from order tracking
        self.order_to_pool.remove(&order_id);

        // Return acknowledgement
        Ok(OrderCancelAcknowledgementBody { order_id })
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    // Helper function to create legs for testing
    fn create_legs(leg_data: &[(u64, bool)]) -> Vec<OrderNewLeg> {
        leg_data
            .iter()
            .map(|(leg_security_id, is_over)| OrderNewLeg {
                leg_security_id: *leg_security_id,
                is_over: *is_over,
            })
            .collect()
    }

    #[test]
    fn test_auto_pool_creation() {
        let mut manager = PoolManager::new();

        // Pool should be created automatically when first order arrives
        assert_eq!(manager.num_pools(), 0);

        let (_eliminations, ack, fills) = manager
            .create_entry(OrderNewBody {
                client_order_id: 1001,
                legs: create_legs(&[(101, false), (102, false)]),
                order_type: OrderType::Limit as i32,
                portion: 250,
                quantity: 250,
                self_match_id: None,
            })
            .unwrap();

        assert_eq!(manager.num_pools(), 1);
        assert_eq!(ack.client_order_id, 1001);
        assert!(ack.order_id > 0);
        assert_eq!(fills.len(), 0);
    }

    #[test]
    fn test_leg_order_independence() {
        let mut manager = PoolManager::new();

        // Create order with legs [101, 102]
        manager
            .create_entry(OrderNewBody {
                client_order_id: 2001,
                legs: create_legs(&[(101, false), (102, true)]),
                order_type: OrderType::Limit as i32,
                portion: 250,
                quantity: 250,
                self_match_id: None,
            })
            .unwrap();

        assert_eq!(manager.num_pools(), 1);

        // Create order with legs [102, 101] - should use same pool
        manager
            .create_entry(OrderNewBody {
                client_order_id: 2002,
                legs: create_legs(&[(102, false), (101, true)]),
                order_type: OrderType::Limit as i32,
                portion: 250,
                quantity: 250,
                self_match_id: None,
            })
            .unwrap();

        assert_eq!(manager.num_pools(), 1); // Still only one pool
    }

    #[test]
    fn test_create_entry_with_fill() {
        let mut manager = PoolManager::new();

        // Submit entries to all 4 lineups (using 250k portions to work with 1M total)
        // Lineup 0: both under (101=false, 102=false)
        let (_eliminations, _ack0, fills0) = manager
            .create_entry(OrderNewBody {
                client_order_id: 3001,
                legs: create_legs(&[(101, false), (102, false)]),
                order_type: OrderType::Limit as i32,
                portion: 250_000,
                quantity: 250_000,
                self_match_id: None,
            })
            .unwrap();
        assert_eq!(fills0.len(), 0);

        // Lineup 1: 101=over, 102=under
        let (_eliminations, _ack1, fills1) = manager
            .create_entry(OrderNewBody {
                client_order_id: 3002,
                legs: create_legs(&[(101, true), (102, false)]),
                order_type: OrderType::Limit as i32,
                portion: 250_000,
                quantity: 250_000,
                self_match_id: None,
            })
            .unwrap();
        assert_eq!(fills1.len(), 0);

        // Lineup 2: 101=under, 102=over
        let (_eliminations, _ack2, fills2) = manager
            .create_entry(OrderNewBody {
                client_order_id: 3003,
                legs: create_legs(&[(101, false), (102, true)]),
                order_type: OrderType::Limit as i32,
                portion: 250_000,
                quantity: 250_000,
                self_match_id: None,
            })
            .unwrap();
        assert_eq!(fills2.len(), 0);

        // Lineup 3: both over (101=true, 102=true) - should trigger fill
        let (_eliminations, _ack3, fills3) = manager
            .create_entry(OrderNewBody {
                client_order_id: 3004,
                legs: create_legs(&[(101, true), (102, true)]),
                order_type: OrderType::Limit as i32,
                portion: 250_000,
                quantity: 250_000,
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

        // Submit passive entries with enough quantity for 2 fills
        manager
            .create_entry(OrderNewBody {
                client_order_id: 4001,
                legs: create_legs(&[(101, false), (102, false)]),
                order_type: OrderType::Limit as i32,
                portion: 250_000,
                quantity: 500_000,
                self_match_id: None,
            })
            .unwrap();

        manager
            .create_entry(OrderNewBody {
                client_order_id: 4002,
                legs: create_legs(&[(101, true), (102, false)]),
                order_type: OrderType::Limit as i32,
                portion: 250_000,
                quantity: 250_000,
                self_match_id: None,
            })
            .unwrap();

        manager
            .create_entry(OrderNewBody {
                client_order_id: 4003,
                legs: create_legs(&[(101, true), (102, false)]),
                order_type: OrderType::Limit as i32,
                portion: 250_000,
                quantity: 250_000,
                self_match_id: None,
            })
            .unwrap();

        manager
            .create_entry(OrderNewBody {
                client_order_id: 4004,
                legs: create_legs(&[(101, false), (102, true)]),
                order_type: OrderType::Limit as i32,
                portion: 250_000,
                quantity: 500_000,
                self_match_id: None,
            })
            .unwrap();

        // Aggressor with enough for 2 fills
        let (_eliminations, _ack, fills) = manager
            .create_entry(OrderNewBody {
                client_order_id: 4005,
                legs: create_legs(&[(101, true), (102, true)]),
                order_type: OrderType::Limit as i32,
                portion: 250_000,
                quantity: 500_000,
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
    fn test_empty_legs_error() {
        let mut manager = PoolManager::new();

        let result = manager.create_entry(OrderNewBody {
            client_order_id: 5001,
            legs: vec![],
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

        // Submit passive entry for lineup 0 (under)
        manager
            .create_entry(OrderNewBody {
                client_order_id: 6001,
                legs: create_legs(&[(101, false)]),
                order_type: OrderType::Limit as i32,
                portion: 600_000,
                quantity: 600_000,
                self_match_id: None,
            })
            .unwrap();

        // Market order for lineup 1 (over) should calculate portion = 400k
        let (_eliminations, _ack, fills) = manager
            .create_entry(OrderNewBody {
                client_order_id: 6002,
                legs: create_legs(&[(101, true)]),
                order_type: OrderType::Market as i32,
                portion: 0, // Ignored for market orders
                quantity: 400_000,
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
        assert_eq!(market_fill.matched_portion, 400_000);
    }

    #[test]
    fn test_cancel_entry_success() {
        let mut manager = PoolManager::new();

        // Create an order
        let (_eliminations, ack, _fills) = manager
            .create_entry(OrderNewBody {
                client_order_id: 7001,
                legs: create_legs(&[(101, false), (102, false)]),
                order_type: OrderType::Limit as i32,
                portion: 250_000,
                quantity: 250_000,
                self_match_id: None,
            })
            .unwrap();

        let order_id = ack.order_id;

        // Verify order is tracked
        assert!(manager.get_pool_key_for_order(order_id).is_some());

        // Cancel the order
        let cancel_ack = manager
            .cancel_entry(OrderCancelBody { order_id })
            .unwrap();

        assert_eq!(cancel_ack.order_id, order_id);

        // Verify order is no longer tracked
        assert!(manager.get_pool_key_for_order(order_id).is_none());

        // Verify entry is removed from pool
        let pool = manager.get_pool(&[101, 102]).unwrap();
        let state = pool.get_state();
        for book in &state.books {
            assert!(!book.entries.iter().any(|e| e.id == order_id));
        }
    }

    #[test]
    fn test_cancel_entry_not_found() {
        let mut manager = PoolManager::new();

        // Try to cancel a non-existent order
        let result = manager.cancel_entry(OrderCancelBody { order_id: 9999 });

        assert!(result.is_err());
        assert!(result.unwrap_err().contains("not found"));
    }

    #[test]
    fn test_cancel_prevents_fill() {
        let mut manager = PoolManager::new();

        // Create orders for 3 out of 4 lineups
        let (_eliminations, ack0, _) = manager
            .create_entry(OrderNewBody {
                client_order_id: 8001,
                legs: create_legs(&[(101, false), (102, false)]),
                order_type: OrderType::Limit as i32,
                portion: 250_000,
                quantity: 250_000,
                self_match_id: None,
            })
            .unwrap();

        let (_eliminations, ack1, _) = manager
            .create_entry(OrderNewBody {
                client_order_id: 8002,
                legs: create_legs(&[(101, true), (102, false)]),
                order_type: OrderType::Limit as i32,
                portion: 250_000,
                quantity: 250_000,
                self_match_id: None,
            })
            .unwrap();

        let (_eliminations, ack2, _) = manager
            .create_entry(OrderNewBody {
                client_order_id: 8003,
                legs: create_legs(&[(101, false), (102, true)]),
                order_type: OrderType::Limit as i32,
                portion: 250_000,
                quantity: 250_000,
                self_match_id: None,
            })
            .unwrap();

        // Cancel one of the orders
        manager
            .cancel_entry(OrderCancelBody {
                order_id: ack1.order_id,
            })
            .unwrap();

        // Now create the 4th order - should NOT trigger a fill
        let (_eliminations, _ack3, fills) = manager
            .create_entry(OrderNewBody {
                client_order_id: 8004,
                legs: create_legs(&[(101, true), (102, true)]),
                order_type: OrderType::Limit as i32,
                portion: 250_000,
                quantity: 250_000,
                self_match_id: None,
            })
            .unwrap();

        // No fills because lineup 1 has no entries (it was cancelled)
        assert_eq!(fills.len(), 0);

        // Verify the other orders are still in the pool
        let pool = manager.get_pool(&[101, 102]).unwrap();
        let state = pool.get_state();
        assert!(state.books[0].entries.iter().any(|e| e.id == ack0.order_id));
        assert!(!state.books[1].entries.iter().any(|e| e.id == ack1.order_id)); // Cancelled
        assert!(state.books[2].entries.iter().any(|e| e.id == ack2.order_id));
    }

    #[test]
    fn test_cancel_after_partial_fill() {
        let mut manager = PoolManager::new();

        // Create orders with quantity for 2 fills
        manager
            .create_entry(OrderNewBody {
                client_order_id: 9001,
                legs: create_legs(&[(101, false), (102, false)]),
                order_type: OrderType::Limit as i32,
                portion: 250_000,
                quantity: 500_000,
                self_match_id: None,
            })
            .unwrap();

        manager
            .create_entry(OrderNewBody {
                client_order_id: 9002,
                legs: create_legs(&[(101, true), (102, false)]),
                order_type: OrderType::Limit as i32,
                portion: 250_000,
                quantity: 250_000,
                self_match_id: None,
            })
            .unwrap();

        manager
            .create_entry(OrderNewBody {
                client_order_id: 9003,
                legs: create_legs(&[(101, true), (102, false)]),
                order_type: OrderType::Limit as i32,
                portion: 250_000,
                quantity: 250_000,
                self_match_id: None,
            })
            .unwrap();

        let (_eliminations, ack2, _) = manager
            .create_entry(OrderNewBody {
                client_order_id: 9004,
                legs: create_legs(&[(101, false), (102, true)]),
                order_type: OrderType::Limit as i32,
                portion: 250_000,
                quantity: 500_000,
                self_match_id: None,
            })
            .unwrap();

        // First order triggers one fill
        let (_eliminations, _ack3, fills) = manager
            .create_entry(OrderNewBody {
                client_order_id: 9005,
                legs: create_legs(&[(101, true), (102, true)]),
                order_type: OrderType::Limit as i32,
                portion: 250_000,
                quantity: 250_000,
                self_match_id: None,
            })
            .unwrap();

        assert_eq!(fills.len(), 1); // One fill occurred

        // Cancel an order that still has quantity remaining
        manager
            .cancel_entry(OrderCancelBody {
                order_id: ack2.order_id,
            })
            .unwrap();

        // Try to trigger another fill - should fail because we cancelled an entry
        let (_eliminations, _ack4, fills2) = manager
            .create_entry(OrderNewBody {
                client_order_id: 9006,
                legs: create_legs(&[(101, true), (102, true)]),
                order_type: OrderType::Limit as i32,
                portion: 250_000,
                quantity: 250_000,
                self_match_id: None,
            })
            .unwrap();

        // No fill because lineup 2 is now empty (cancelled)
        assert_eq!(fills2.len(), 0);
    }

    #[test]
    fn test_eliminations_zero() {
        let mut manager = PoolManager::new();

        // Submit entries with different self_match_ids - no eliminations should occur
        manager
            .create_entry(OrderNewBody {
                client_order_id: 1001,
                legs: create_legs(&[(101, false), (102, false)]),
                order_type: OrderType::Limit as i32,
                portion: 250_000,
                quantity: 250_000,
                self_match_id: Some(1),
            })
            .unwrap();

        manager
            .create_entry(OrderNewBody {
                client_order_id: 1002,
                legs: create_legs(&[(101, true), (102, false)]),
                order_type: OrderType::Limit as i32,
                portion: 250_000,
                quantity: 250_000,
                self_match_id: Some(2),
            })
            .unwrap();

        // Submit entry with different self_match_id - should have 0 eliminations
        let (eliminations, _ack, _fills) = manager
            .create_entry(OrderNewBody {
                client_order_id: 1003,
                legs: create_legs(&[(101, false), (102, true)]),
                order_type: OrderType::Limit as i32,
                portion: 250_000,
                quantity: 250_000,
                self_match_id: Some(3),
            })
            .unwrap();

        assert_eq!(eliminations.len(), 0);
    }

    #[test]
    fn test_eliminations_one() {
        let mut manager = PoolManager::new();

        // Submit entry with self_match_id=42
        let (_eliminations, ack1, _fills) = manager
            .create_entry(OrderNewBody {
                client_order_id: 2001,
                legs: create_legs(&[(101, false), (102, false)]),
                order_type: OrderType::Limit as i32,
                portion: 250_000,
                quantity: 250_000,
                self_match_id: Some(42),
            })
            .unwrap();

        // Submit entry to different lineup with same self_match_id - should eliminate the first
        let (eliminations, _ack2, _fills) = manager
            .create_entry(OrderNewBody {
                client_order_id: 2002,
                legs: create_legs(&[(101, true), (102, false)]),
                order_type: OrderType::Limit as i32,
                portion: 250_000,
                quantity: 250_000,
                self_match_id: Some(42),
            })
            .unwrap();

        assert_eq!(eliminations.len(), 1);
        assert_eq!(eliminations[0].order_id, ack1.order_id);
        assert_eq!(eliminations[0].elimination_description, "Eliminated due to self-match prevention");
    }

    #[test]
    fn test_eliminations_two() {
        let mut manager = PoolManager::new();

        // Key insight: entries in the SAME lineup with the same self_match_id are allowed to coexist.
        // So we can put 2 entries in lineup 0 with self_match_id=77, then submit to lineup 1 with
        // self_match_id=77, which will eliminate both entries in lineup 0.

        // Submit two entries to lineup 0 (same lineup) with self_match_id=77
        let (_eliminations, ack1, _fills) = manager
            .create_entry(OrderNewBody {
                client_order_id: 3001,
                legs: create_legs(&[(101, false), (102, false)]),
                order_type: OrderType::Limit as i32,
                portion: 250_000,
                quantity: 250_000,
                self_match_id: Some(77),
            })
            .unwrap();

        let (_eliminations, ack2, _fills) = manager
            .create_entry(OrderNewBody {
                client_order_id: 3002,
                legs: create_legs(&[(101, false), (102, false)]),
                order_type: OrderType::Limit as i32,
                portion: 200_000,
                quantity: 200_000,
                self_match_id: Some(77),
            })
            .unwrap();

        // Both ack1 and ack2 should be in lineup 0 (same lineup allows same self_match_id)
        // Now submit entry to lineup 1 (different lineup) with same self_match_id
        // This should eliminate both ack1 and ack2
        let (eliminations, _ack3, _fills) = manager
            .create_entry(OrderNewBody {
                client_order_id: 3003,
                legs: create_legs(&[(101, true), (102, false)]),
                order_type: OrderType::Limit as i32,
                portion: 250_000,
                quantity: 250_000,
                self_match_id: Some(77),
            })
            .unwrap();

        assert_eq!(eliminations.len(), 2);
        // Check that both order IDs are in the eliminations
        let eliminated_ids: Vec<u64> = eliminations.iter().map(|e| e.order_id).collect();
        assert!(eliminated_ids.contains(&ack1.order_id));
        assert!(eliminated_ids.contains(&ack2.order_id));
        // Check elimination descriptions
        assert_eq!(eliminations[0].elimination_description, "Eliminated due to self-match prevention");
        assert_eq!(eliminations[1].elimination_description, "Eliminated due to self-match prevention");
    }
}
