// pool_manager.rs
//
// PoolManager coordinates multiple EntryPools and handles the translation between
// protobuf messages and the EntryPool API.

use std::collections::HashMap;

// Import the EntryPool types
use crate::entry_pool::{EntryPool, EntryParameters, EntryType};

// Import pool utilities
use crate::pool_utils::{create_pool_key, calculate_lineup_index, uuid_to_u128, Leg};

// Import protobuf generated types
use crate::matching_service_package::{
    new_order::Body as NewOrderBody,
    new_order_acknowledgement::Body as NewOrderAcknowledgementBody,
    cancel_order::Body as CancelOrderBody,
    cancel_order_acknowledgement::Body as CancelOrderAcknowledgementBody,
    order_elimination::Body as OrderEliminationBody,
    r#match::Body as MatchBody,
    r#match::body::FillEvent,
    OrderType
};

const TOTAL_UNITS: u64 = 1_000_000;

/// Manages multiple entry pools and coordinates order routing
pub struct PoolManager {
    /// Map from sorted leg_security_ids to EntryPool
    pools: HashMap<Vec<u128>, PoolInfo>,
    /// Map from order_id to pool key for order cancellation
    order_to_pool: HashMap<u64, Vec<u128>>,
    /// Counter for generating unique order IDs across all pools
    next_order_id: u64,
    /// Counter for generating unique transaction IDs
    next_transaction_id: u64,
    /// Counter for generating unique match IDs
    next_match_id: u64,
    /// Counter for generating unique fill event IDs
    next_fill_event_id: u64,
}

/// Information about a pool
struct PoolInfo {
    pool: EntryPool,
}

impl PoolManager {
    /// Creates a new PoolManager
    pub fn new() -> Self {
        PoolManager {
            pools: HashMap::new(),
            order_to_pool: HashMap::new(),
            next_order_id: 1,
            next_transaction_id: 1,
            next_match_id: 1,
            next_fill_event_id: 1,
        }
    }

    /// Creates a new entry/order and returns eliminations, acknowledgement, and any matches
    /// Automatically creates the pool if it doesn't exist
    pub fn create_entry(
        &mut self,
        order: NewOrderBody,
    ) -> Result<(Vec<OrderEliminationBody>, NewOrderAcknowledgementBody, Vec<MatchBody>, Vec<OrderEliminationBody>), String> {
        // Validate we have at least one leg
        if order.legs.is_empty() {
            return Err("Order must have at least one leg".to_string());
        }

        // Extract leg security IDs and convert to internal Leg format
        let leg_security_ids: Vec<u128> = order.legs.iter().map(|l| {
            l.leg_security_id.as_ref().map_or(0u128, uuid_to_u128)
        }).collect();
        let pool_key = create_pool_key(&leg_security_ids);

        // Convert protobuf legs to internal Leg format
        let legs: Vec<Leg> = order.legs.iter().map(|l| Leg {
            leg_security_id: l.leg_security_id.as_ref().map_or(0u128, uuid_to_u128),
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
                PoolInfo { pool },
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

        // Convert optional UUID self_match_id to u128
        let self_match_id = order.self_match_id.as_ref().map(uuid_to_u128);

        // Create entry parameters
        let params = EntryParameters {
            entry_id: order_id,
            entry_type,
            portion: order.portion,
            quantity: order.quantity,
            self_match_id,
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

        // Create elimination bodies for cancelled market entries
        let market_entry_elimination_bodies: Vec<OrderEliminationBody> = submit_result
            .cancelled_market_entry_ids
            .iter()
            .map(|&cancelled_id| OrderEliminationBody {
                order_id: cancelled_id,
                elimination_description: "Market order removed (not fully filled)".to_string(),
            })
            .collect();

        // Create acknowledgement
        let ack = NewOrderAcknowledgementBody {
            client_order_id: order.client_order_id,
            order_id
        };

        // Convert matches to match bodies
        let transaction_id = if !submit_result.match_infos.is_empty() {
            let tid = self.next_transaction_id;
            self.next_transaction_id += 1;
            tid
        } else {
            0 // Not used if no matches
        };

        let mut match_bodies = Vec::new();
        for match_info in submit_result.match_infos {
            let match_id = self.next_match_id;
            self.next_match_id += 1;

            let mut fill_events = Vec::new();
            for filled_entry in match_info.filled_entries {
                let fill_event_id = self.next_fill_event_id;
                self.next_fill_event_id += 1;

                fill_events.push(FillEvent {
                    fill_event_id,
                    order_id: filled_entry.entry.id,
                    is_aggressor: filled_entry.entry.lineup_index
                        == match_info.aggressor_lineup_index,
                    matched_portion: filled_entry.matched_portion,
                    is_complete: filled_entry.is_complete,
                });
            }

            match_bodies.push(MatchBody {
                transaction_id,
                match_id,
                matched_quantity: match_info.matched_quantity,
                fill_events,
            });
        }

        Ok((elimination_bodies, ack, match_bodies, market_entry_elimination_bodies))
    }

    /// Returns the number of pools currently managed
    pub fn num_pools(&self) -> usize {
        self.pools.len()
    }

    /// Gets a reference to a specific pool's EntryPool by leg security IDs (for testing/debugging)
    pub fn get_pool(&self, leg_security_ids: &[u128]) -> Option<&EntryPool> {
        let pool_key = create_pool_key(leg_security_ids);
        self.pools.get(&pool_key).map(|info| &info.pool)
    }

    /// Gets the pool key for a specific order ID (for testing/debugging)
    pub fn get_pool_key_for_order(&self, order_id: u64) -> Option<&Vec<u128>> {
        self.order_to_pool.get(&order_id)
    }

    /// Cancels an existing order
    ///
    /// # Arguments
    /// * `cancel` - The order cancel request containing the order_id
    ///
    /// # Returns
    /// * `Ok(CancelOrderAcknowledgementBody)` - Successful cancellation
    /// * `Err(String)` - Error message if cancellation failed
    pub fn cancel_entry(
        &mut self,
        cancel: CancelOrderBody,
    ) -> Result<CancelOrderAcknowledgementBody, String> {
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
        Ok(CancelOrderAcknowledgementBody { order_id })
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::matching_service_package::new_order::body::Leg as NewOrderLeg;

    // Helper function to create a UUID self_match_id from a simple integer
    fn smid(id: u128) -> Option<crate::common::Uuid> {
        Some(crate::common::Uuid {
            upper: (id >> 64) as u64,
            lower: id as u64,
        })
    }

    // Helper function to create a UUID client_order_id from a u128
    fn coid(id: u128) -> Option<crate::common::Uuid> {
        Some(crate::common::Uuid {
            upper: (id >> 64) as u64,
            lower: id as u64,
        })
    }

    // Helper function to create a UUID from a simple u128 (for leg_security_id)
    fn lsid(id: u128) -> Option<crate::common::Uuid> {
        Some(crate::common::Uuid {
            upper: (id >> 64) as u64,
            lower: id as u64,
        })
    }

    // Helper function to create legs for testing
    fn create_legs(leg_data: &[(u128, bool)]) -> Vec<NewOrderLeg> {
        leg_data
            .iter()
            .map(|(leg_security_id, is_over)| NewOrderLeg {
                leg_security_id: lsid(*leg_security_id),
                is_over: *is_over,
            })
            .collect()
    }

    #[test]
    fn test_auto_pool_creation() {
        let mut manager = PoolManager::new();

        // Pool should be created automatically when first order arrives
        assert_eq!(manager.num_pools(), 0);

        let (_eliminations, ack, matches, _market_eliminations) = manager
            .create_entry(NewOrderBody {
                client_order_id: coid(1001),
                legs: create_legs(&[(101, false), (102, false)]),
                order_type: OrderType::Limit as i32,
                portion: 250,
                quantity: 250,
                self_match_id: None,
            })
            .unwrap();

        assert_eq!(manager.num_pools(), 1);
        assert_eq!(ack.client_order_id, coid(1001));
        assert!(ack.order_id > 0);
        assert_eq!(matches.len(), 0);
    }

    #[test]
    fn test_leg_order_independence() {
        let mut manager = PoolManager::new();

        // Create order with legs [101, 102]
        manager
            .create_entry(NewOrderBody {
                client_order_id: coid(2001),
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
            .create_entry(NewOrderBody {
                client_order_id: coid(2002),
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
        let (_eliminations, _ack0, matches0, _market_eliminations) = manager
            .create_entry(NewOrderBody {
                client_order_id: coid(3001),
                legs: create_legs(&[(101, false), (102, false)]),
                order_type: OrderType::Limit as i32,
                portion: 250_000,
                quantity: 250_000,
                self_match_id: None,
            })
            .unwrap();
        assert_eq!(matches0.len(), 0);

        // Lineup 1: 101=over, 102=under
        let (_eliminations, _ack1, matches1, _market_eliminations) = manager
            .create_entry(NewOrderBody {
                client_order_id: coid(3002),
                legs: create_legs(&[(101, true), (102, false)]),
                order_type: OrderType::Limit as i32,
                portion: 250_000,
                quantity: 250_000,
                self_match_id: None,
            })
            .unwrap();
        assert_eq!(matches1.len(), 0);

        // Lineup 2: 101=under, 102=over
        let (_eliminations, _ack2, matches2, _market_eliminations) = manager
            .create_entry(NewOrderBody {
                client_order_id: coid(3003),
                legs: create_legs(&[(101, false), (102, true)]),
                order_type: OrderType::Limit as i32,
                portion: 250_000,
                quantity: 250_000,
                self_match_id: None,
            })
            .unwrap();
        assert_eq!(matches2.len(), 0);

        // Lineup 3: both over (101=true, 102=true) - should trigger match
        let (_eliminations, _ack3, matches3, _market_eliminations) = manager
            .create_entry(NewOrderBody {
                client_order_id: coid(3004),
                legs: create_legs(&[(101, true), (102, true)]),
                order_type: OrderType::Limit as i32,
                portion: 250_000,
                quantity: 250_000,
                self_match_id: None,
            })
            .unwrap();

        assert_eq!(matches3.len(), 1);
        assert_eq!(matches3[0].fill_events.len(), 4); // One fill event per lineup

        // Verify transaction ID is shared
        let transaction_id = matches3[0].transaction_id;
        assert!(transaction_id > 0);

        // Verify each fill event has unique fill_event_id
        let fill_event_ids: Vec<u64> = matches3[0].fill_events.iter().map(|f| f.fill_event_id).collect();
        assert_eq!(fill_event_ids.len(), 4);
        for i in 0..4 {
            for j in (i + 1)..4 {
                assert_ne!(fill_event_ids[i], fill_event_ids[j]);
            }
        }

        // Verify aggressor is marked
        let aggressor_count = matches3[0].fill_events.iter().filter(|f| f.is_aggressor).count();
        assert_eq!(aggressor_count, 1);
    }

    #[test]
    fn test_multiple_matches_share_transaction_id() {
        let mut manager = PoolManager::new();

        // Submit passive entries with enough quantity for 2 matches
        manager
            .create_entry(NewOrderBody {
                client_order_id: coid(4001),
                legs: create_legs(&[(101, false), (102, false)]),
                order_type: OrderType::Limit as i32,
                portion: 250_000,
                quantity: 500_000,
                self_match_id: None,
            })
            .unwrap();

        manager
            .create_entry(NewOrderBody {
                client_order_id: coid(4002),
                legs: create_legs(&[(101, true), (102, false)]),
                order_type: OrderType::Limit as i32,
                portion: 250_000,
                quantity: 250_000,
                self_match_id: None,
            })
            .unwrap();

        manager
            .create_entry(NewOrderBody {
                client_order_id: coid(4003),
                legs: create_legs(&[(101, true), (102, false)]),
                order_type: OrderType::Limit as i32,
                portion: 250_000,
                quantity: 250_000,
                self_match_id: None,
            })
            .unwrap();

        manager
            .create_entry(NewOrderBody {
                client_order_id: coid(4004),
                legs: create_legs(&[(101, false), (102, true)]),
                order_type: OrderType::Limit as i32,
                portion: 250_000,
                quantity: 500_000,
                self_match_id: None,
            })
            .unwrap();

        // Aggressor with enough for 2 matches
        let (_eliminations, _ack, matches, _market_eliminations) = manager
            .create_entry(NewOrderBody {
                client_order_id: coid(4005),
                legs: create_legs(&[(101, true), (102, true)]),
                order_type: OrderType::Limit as i32,
                portion: 250_000,
                quantity: 500_000,
                self_match_id: None,
            })
            .unwrap();

        assert_eq!(matches.len(), 2);

        // Both matches should share the same transaction_id
        assert_eq!(matches[0].transaction_id, matches[1].transaction_id);

        // But have different match_ids
        assert_ne!(matches[0].match_id, matches[1].match_id);
    }

    #[test]
    fn test_empty_legs_error() {
        let mut manager = PoolManager::new();

        let result = manager.create_entry(NewOrderBody {
            client_order_id: coid(5001),
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
            .create_entry(NewOrderBody {
                client_order_id: coid(6001),
                legs: create_legs(&[(101, false)]),
                order_type: OrderType::Limit as i32,
                portion: 600_000,
                quantity: 600_000,
                self_match_id: None,
            })
            .unwrap();

        // Market order for lineup 1 (over) should calculate portion = 400k
        let (_eliminations, _ack, matches, market_eliminations) = manager
            .create_entry(NewOrderBody {
                client_order_id: coid(6002),
                legs: create_legs(&[(101, true)]),
                order_type: OrderType::Market as i32,
                portion: 0, // Ignored for market orders
                quantity: 400_000,
                self_match_id: None,
            })
            .unwrap();

        assert_eq!(matches.len(), 1);
        assert_eq!(matches[0].fill_events.len(), 2);

        // Find the market order fill event
        let market_fill_event = matches[0]
            .fill_events
            .iter()
            .find(|f| f.is_aggressor)
            .unwrap();
        assert_eq!(market_fill_event.matched_portion, 400_000);

        // Market order was fully filled, so no market entry eliminations
        assert_eq!(market_eliminations.len(), 0);
    }

    #[test]
    fn test_market_order_partial_fill() {
        let mut manager = PoolManager::new();

        // Submit passive entries with quantity=1 each
        manager
            .create_entry(NewOrderBody {
                client_order_id: coid(6101),
                legs: create_legs(&[(101, false)]),
                order_type: OrderType::Limit as i32,
                portion: 600_000,
                quantity: 600_000,
                self_match_id: None,
            })
            .unwrap();

        // Market order with quantity=3 should only match once (limited by passive quantity)
        let (_eliminations, ack, matches, market_eliminations) = manager
            .create_entry(NewOrderBody {
                client_order_id: coid(6102),
                legs: create_legs(&[(101, true)]),
                order_type: OrderType::Market as i32,
                portion: 0, // Ignored for market orders
                quantity: 1_800_000, // 3x the matched quantity
                self_match_id: None,
            })
            .unwrap();

        // Should have 1 match
        assert_eq!(matches.len(), 1);
        assert_eq!(matches[0].matched_quantity, 600_000);

        // Market order had remaining quantity, so it should be in market_eliminations
        assert_eq!(market_eliminations.len(), 1);
        assert_eq!(market_eliminations[0].order_id, ack.order_id);
        assert_eq!(market_eliminations[0].elimination_description, "Market order removed (not fully filled)");
    }

    #[test]
    fn test_cancel_entry_success() {
        let mut manager = PoolManager::new();

        // Create an order
        let (_eliminations, ack, _matches, _market_eliminations) = manager
            .create_entry(NewOrderBody {
                client_order_id: coid(7001),
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
            .cancel_entry(CancelOrderBody { order_id })
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
        let result = manager.cancel_entry(CancelOrderBody { order_id: 9999 });

        assert!(result.is_err());
        assert!(result.unwrap_err().contains("not found"));
    }

    #[test]
    fn test_cancel_prevents_fill() {
        let mut manager = PoolManager::new();

        // Create orders for 3 out of 4 lineups
        let (_eliminations, ack0, _, _market_eliminations) = manager
            .create_entry(NewOrderBody {
                client_order_id: coid(8001),
                legs: create_legs(&[(101, false), (102, false)]),
                order_type: OrderType::Limit as i32,
                portion: 250_000,
                quantity: 250_000,
                self_match_id: None,
            })
            .unwrap();

        let (_eliminations, ack1, _, _market_eliminations) = manager
            .create_entry(NewOrderBody {
                client_order_id: coid(8002),
                legs: create_legs(&[(101, true), (102, false)]),
                order_type: OrderType::Limit as i32,
                portion: 250_000,
                quantity: 250_000,
                self_match_id: None,
            })
            .unwrap();

        let (_eliminations, ack2, _, _market_eliminations) = manager
            .create_entry(NewOrderBody {
                client_order_id: coid(8003),
                legs: create_legs(&[(101, false), (102, true)]),
                order_type: OrderType::Limit as i32,
                portion: 250_000,
                quantity: 250_000,
                self_match_id: None,
            })
            .unwrap();

        // Cancel one of the orders
        manager
            .cancel_entry(CancelOrderBody {
                order_id: ack1.order_id,
            })
            .unwrap();

        // Now create the 4th order - should NOT trigger a match
        let (_eliminations, _ack3, matches, _market_eliminations) = manager
            .create_entry(NewOrderBody {
                client_order_id: coid(8004),
                legs: create_legs(&[(101, true), (102, true)]),
                order_type: OrderType::Limit as i32,
                portion: 250_000,
                quantity: 250_000,
                self_match_id: None,
            })
            .unwrap();

        // No matches because lineup 1 has no entries (it was cancelled)
        assert_eq!(matches.len(), 0);

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
            .create_entry(NewOrderBody {
                client_order_id: coid(9001),
                legs: create_legs(&[(101, false), (102, false)]),
                order_type: OrderType::Limit as i32,
                portion: 250_000,
                quantity: 500_000,
                self_match_id: None,
            })
            .unwrap();

        manager
            .create_entry(NewOrderBody {
                client_order_id: coid(9002),
                legs: create_legs(&[(101, true), (102, false)]),
                order_type: OrderType::Limit as i32,
                portion: 250_000,
                quantity: 250_000,
                self_match_id: None,
            })
            .unwrap();

        manager
            .create_entry(NewOrderBody {
                client_order_id: coid(9003),
                legs: create_legs(&[(101, true), (102, false)]),
                order_type: OrderType::Limit as i32,
                portion: 250_000,
                quantity: 250_000,
                self_match_id: None,
            })
            .unwrap();

        let (_eliminations, ack2, _, _market_eliminations) = manager
            .create_entry(NewOrderBody {
                client_order_id: coid(9004),
                legs: create_legs(&[(101, false), (102, true)]),
                order_type: OrderType::Limit as i32,
                portion: 250_000,
                quantity: 500_000,
                self_match_id: None,
            })
            .unwrap();

        // First order triggers one match
        let (_eliminations, _ack3, matches, _market_eliminations) = manager
            .create_entry(NewOrderBody {
                client_order_id: coid(9005),
                legs: create_legs(&[(101, true), (102, true)]),
                order_type: OrderType::Limit as i32,
                portion: 250_000,
                quantity: 250_000,
                self_match_id: None,
            })
            .unwrap();

        assert_eq!(matches.len(), 1); // One match occurred

        // Cancel an order that still has quantity remaining
        manager
            .cancel_entry(CancelOrderBody {
                order_id: ack2.order_id,
            })
            .unwrap();

        // Try to trigger another match - should fail because we cancelled an entry
        let (_eliminations, _ack4, matches2, _market_eliminations) = manager
            .create_entry(NewOrderBody {
                client_order_id: coid(9006),
                legs: create_legs(&[(101, true), (102, true)]),
                order_type: OrderType::Limit as i32,
                portion: 250_000,
                quantity: 250_000,
                self_match_id: None,
            })
            .unwrap();

        // No match because lineup 2 is now empty (cancelled)
        assert_eq!(matches2.len(), 0);
    }

    #[test]
    fn test_eliminations_zero() {
        let mut manager = PoolManager::new();

        // Submit entries with different self_match_ids - no eliminations should occur
        manager
            .create_entry(NewOrderBody {
                client_order_id: coid(1001),
                legs: create_legs(&[(101, false), (102, false)]),
                order_type: OrderType::Limit as i32,
                portion: 250_000,
                quantity: 250_000,
                self_match_id: smid(1),
            })
            .unwrap();

        manager
            .create_entry(NewOrderBody {
                client_order_id: coid(1002),
                legs: create_legs(&[(101, true), (102, false)]),
                order_type: OrderType::Limit as i32,
                portion: 250_000,
                quantity: 250_000,
                self_match_id: smid(2),
            })
            .unwrap();

        // Submit entry with different self_match_id - should have 0 eliminations
        let (eliminations, _ack, _matches, _market_eliminations) = manager
            .create_entry(NewOrderBody {
                client_order_id: coid(1003),
                legs: create_legs(&[(101, false), (102, true)]),
                order_type: OrderType::Limit as i32,
                portion: 250_000,
                quantity: 250_000,
                self_match_id: smid(3),
            })
            .unwrap();

        assert_eq!(eliminations.len(), 0);
    }

    #[test]
    fn test_eliminations_one() {
        let mut manager = PoolManager::new();

        // Submit entry with self_match_id=42
        let (_eliminations, ack1, _matches, _market_eliminations) = manager
            .create_entry(NewOrderBody {
                client_order_id: coid(2001),
                legs: create_legs(&[(101, false), (102, false)]),
                order_type: OrderType::Limit as i32,
                portion: 250_000,
                quantity: 250_000,
                self_match_id: smid(42),
            })
            .unwrap();

        // Submit entry to different lineup with same self_match_id - should eliminate the first
        let (eliminations, _ack2, _matches, _market_eliminations) = manager
            .create_entry(NewOrderBody {
                client_order_id: coid(2002),
                legs: create_legs(&[(101, true), (102, false)]),
                order_type: OrderType::Limit as i32,
                portion: 250_000,
                quantity: 250_000,
                self_match_id: smid(42),
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
        let (_eliminations, ack1, _matches, _market_eliminations) = manager
            .create_entry(NewOrderBody {
                client_order_id: coid(3001),
                legs: create_legs(&[(101, false), (102, false)]),
                order_type: OrderType::Limit as i32,
                portion: 250_000,
                quantity: 250_000,
                self_match_id: smid(77),
            })
            .unwrap();

        let (_eliminations, ack2, _matches, _market_eliminations) = manager
            .create_entry(NewOrderBody {
                client_order_id: coid(3002),
                legs: create_legs(&[(101, false), (102, false)]),
                order_type: OrderType::Limit as i32,
                portion: 200_000,
                quantity: 200_000,
                self_match_id: smid(77),
            })
            .unwrap();

        // Both ack1 and ack2 should be in lineup 0 (same lineup allows same self_match_id)
        // Now submit entry to lineup 1 (different lineup) with same self_match_id
        // This should eliminate both ack1 and ack2
        let (eliminations, _ack3, _matches, _market_eliminations) = manager
            .create_entry(NewOrderBody {
                client_order_id: coid(3003),
                legs: create_legs(&[(101, true), (102, false)]),
                order_type: OrderType::Limit as i32,
                portion: 250_000,
                quantity: 250_000,
                self_match_id: smid(77),
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
