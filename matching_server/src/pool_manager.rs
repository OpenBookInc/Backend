// pool_manager.rs
//
// PoolManager coordinates multiple EntryPools and handles the translation between
// protobuf messages and the EntryPool API.
// Pools must be explicitly defined via DefinePool before orders can be submitted.

use std::collections::HashMap;

// Import the EntryPool types
use crate::entry_pool::{EntryPool, EntryParameters, EntryType};

// Import pool utilities
use crate::pool_utils::uuid_to_u128;

// Import protobuf generated types
use crate::matching_service_package::{
    new_order::Body as NewOrderBody,
    new_order_acknowledgement::Body as NewOrderAcknowledgementBody,
    cancel_order::Body as CancelOrderBody,
    cancel_order_acknowledgement::Body as CancelOrderAcknowledgementBody,
    order_elimination::Body as OrderEliminationBody,
    r#match::Body as MatchBody,
    r#match::body::FillEvent,
    define_pool::Body as DefinePoolBody,
    define_pool_acknowledgement::Body as DefinePoolAcknowledgementBody,
    OrderType
};

/// Manages multiple entry pools and coordinates order routing
pub struct PoolManager {
    /// Map from slate_id (as u128) to PoolInfo
    pools: HashMap<u128, PoolInfo>,
    /// Map from order_id to slate_id for order cancellation
    order_to_pool: HashMap<u64, u128>,
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

    /// Defines a new pool. Must be called before any orders can be submitted to this pool.
    /// Returns an error if a pool with the same slate_id already exists.
    pub fn define_pool(
        &mut self,
        definition: DefinePoolBody,
    ) -> Result<DefinePoolAcknowledgementBody, String> {
        let slate_id = definition.slate_id.as_ref()
            .ok_or_else(|| "Missing slate_id in DefinePool".to_string())?;
        let slate_id_u128 = uuid_to_u128(slate_id);

        if self.pools.contains_key(&slate_id_u128) {
            return Err(format!("Pool already defined for slate_id {:032x}", slate_id_u128));
        }

        let pool = EntryPool::new(definition.total_units, definition.num_lineups as usize);
        self.pools.insert(slate_id_u128, PoolInfo { pool });

        Ok(DefinePoolAcknowledgementBody {
            slate_id: definition.slate_id,
        })
    }

    /// Creates a new entry/order and returns eliminations, acknowledgement, and any matches.
    /// The pool must have been defined via `define_pool` before calling this.
    pub fn create_entry(
        &mut self,
        order: NewOrderBody,
    ) -> Result<(Vec<OrderEliminationBody>, NewOrderAcknowledgementBody, Vec<MatchBody>, Vec<OrderEliminationBody>), String> {
        // Look up pool by slate_id
        let slate_id = order.slate_id.as_ref()
            .ok_or_else(|| "Missing slate_id in NewOrder".to_string())?;
        let slate_id_u128 = uuid_to_u128(slate_id);

        if !self.pools.contains_key(&slate_id_u128) {
            return Err(format!("No pool defined for slate_id {:032x}", slate_id_u128));
        }

        let pool_info = self.pools.get_mut(&slate_id_u128).unwrap();

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
        self.order_to_pool.insert(order_id, slate_id_u128);

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

        // Submit to the entry pool using lineup_index from the order
        let submit_result = pool_info
            .pool
            .submit_entry(order.lineup_index as usize, params)
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

    /// Gets a reference to a specific pool's EntryPool by slate_id (for testing/debugging)
    pub fn get_pool(&self, slate_id: u128) -> Option<&EntryPool> {
        self.pools.get(&slate_id).map(|info| &info.pool)
    }

    /// Gets the slate_id for a specific order ID (for testing/debugging)
    pub fn get_slate_id_for_order(&self, order_id: u64) -> Option<u128> {
        self.order_to_pool.get(&order_id).copied()
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
        let slate_id = self
            .order_to_pool
            .get(&order_id)
            .ok_or_else(|| format!("Order ID {} not found", order_id))?
            .clone();

        // Get the pool
        let pool_info = self
            .pools
            .get_mut(&slate_id)
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

    // Helper function to create a UUID from a simple integer
    fn uuid(id: u128) -> Option<crate::common::Uuid> {
        Some(crate::common::Uuid {
            upper: (id >> 64) as u64,
            lower: id as u64,
        })
    }

    // Convenience: define a 2-leg pool (4 lineups) with 1_000_000 total units
    fn define_standard_pool(manager: &mut PoolManager, slate_id: u128) {
        manager.define_pool(DefinePoolBody {
            slate_id: uuid(slate_id),
            total_units: 1_000_000,
            num_lineups: 4,
        }).unwrap();
    }

    // Convenience: define a 1-leg pool (2 lineups) with 1_000_000 total units
    fn define_single_leg_pool(manager: &mut PoolManager, slate_id: u128) {
        manager.define_pool(DefinePoolBody {
            slate_id: uuid(slate_id),
            total_units: 1_000_000,
            num_lineups: 2,
        }).unwrap();
    }

    fn new_order(slate_id: u128, lineup_index: u64, client_order_id: u128, portion: u64, quantity: u64) -> NewOrderBody {
        NewOrderBody {
            client_order_id: uuid(client_order_id),
            slate_id: uuid(slate_id),
            lineup_index,
            order_type: OrderType::Limit as i32,
            portion,
            quantity,
            self_match_id: None,
        }
    }

    fn new_market_order(slate_id: u128, lineup_index: u64, client_order_id: u128, quantity: u64) -> NewOrderBody {
        NewOrderBody {
            client_order_id: uuid(client_order_id),
            slate_id: uuid(slate_id),
            lineup_index,
            order_type: OrderType::Market as i32,
            portion: 0,
            quantity,
            self_match_id: None,
        }
    }

    fn new_order_with_smid(slate_id: u128, lineup_index: u64, client_order_id: u128, portion: u64, quantity: u64, self_match_id: u128) -> NewOrderBody {
        NewOrderBody {
            client_order_id: uuid(client_order_id),
            slate_id: uuid(slate_id),
            lineup_index,
            order_type: OrderType::Limit as i32,
            portion,
            quantity,
            self_match_id: uuid(self_match_id),
        }
    }

    // --- DefinePool tests ---

    #[test]
    fn test_define_pool_success() {
        let mut manager = PoolManager::new();
        assert_eq!(manager.num_pools(), 0);

        let result = manager.define_pool(DefinePoolBody {
            slate_id: uuid(1),
            total_units: 1_000_000,
            num_lineups: 4,
        });

        assert!(result.is_ok());
        assert_eq!(manager.num_pools(), 1);
    }

    #[test]
    fn test_define_pool_duplicate_rejected() {
        let mut manager = PoolManager::new();

        manager.define_pool(DefinePoolBody {
            slate_id: uuid(1),
            total_units: 1_000_000,
            num_lineups: 4,
        }).unwrap();

        // Duplicate — even with identical fields — should be rejected
        let result = manager.define_pool(DefinePoolBody {
            slate_id: uuid(1),
            total_units: 1_000_000,
            num_lineups: 4,
        });

        assert!(result.is_err());
        assert!(result.unwrap_err().contains("already defined"));
        assert_eq!(manager.num_pools(), 1);
    }

    #[test]
    fn test_define_pool_missing_slate_id() {
        let mut manager = PoolManager::new();

        let result = manager.define_pool(DefinePoolBody {
            slate_id: None,
            total_units: 1_000_000,
            num_lineups: 4,
        });

        assert!(result.is_err());
        assert!(result.unwrap_err().contains("Missing slate_id"));
    }

    #[test]
    fn test_order_rejected_for_undefined_pool() {
        let mut manager = PoolManager::new();

        let result = manager.create_entry(new_order(999, 0, 1001, 250_000, 250_000));

        assert!(result.is_err());
        assert!(result.unwrap_err().contains("No pool defined"));
    }

    // --- Order lifecycle tests ---

    #[test]
    fn test_create_entry_after_define() {
        let mut manager = PoolManager::new();
        define_standard_pool(&mut manager, 1);

        let (_eliminations, ack, matches, _market_eliminations) = manager
            .create_entry(new_order(1, 0, 1001, 250_000, 250_000))
            .unwrap();

        assert_eq!(ack.client_order_id, uuid(1001));
        assert!(ack.order_id > 0);
        assert_eq!(matches.len(), 0);
    }

    #[test]
    fn test_create_entry_with_fill() {
        let mut manager = PoolManager::new();
        define_standard_pool(&mut manager, 1);

        // Submit entries to all 4 lineups (250k portions to work with 1M total)
        // Lineup 0
        let (_e, _a, matches0, _me) = manager
            .create_entry(new_order(1, 0, 3001, 250_000, 250_000))
            .unwrap();
        assert_eq!(matches0.len(), 0);

        // Lineup 1
        let (_e, _a, matches1, _me) = manager
            .create_entry(new_order(1, 1, 3002, 250_000, 250_000))
            .unwrap();
        assert_eq!(matches1.len(), 0);

        // Lineup 2
        let (_e, _a, matches2, _me) = manager
            .create_entry(new_order(1, 2, 3003, 250_000, 250_000))
            .unwrap();
        assert_eq!(matches2.len(), 0);

        // Lineup 3 — should trigger match
        let (_e, _a, matches3, _me) = manager
            .create_entry(new_order(1, 3, 3004, 250_000, 250_000))
            .unwrap();

        assert_eq!(matches3.len(), 1);
        assert_eq!(matches3[0].fill_events.len(), 4); // One fill event per lineup

        // Verify transaction ID is shared
        assert!(matches3[0].transaction_id > 0);

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
        define_standard_pool(&mut manager, 1);

        // Submit passive entries with enough quantity for 2 matches
        manager.create_entry(new_order(1, 0, 4001, 250_000, 500_000)).unwrap();
        manager.create_entry(new_order(1, 1, 4002, 250_000, 250_000)).unwrap();
        manager.create_entry(new_order(1, 1, 4003, 250_000, 250_000)).unwrap();
        manager.create_entry(new_order(1, 2, 4004, 250_000, 500_000)).unwrap();

        // Aggressor with enough for 2 matches
        let (_e, _a, matches, _me) = manager
            .create_entry(new_order(1, 3, 4005, 250_000, 500_000))
            .unwrap();

        assert_eq!(matches.len(), 2);
        assert_eq!(matches[0].transaction_id, matches[1].transaction_id);
        assert_ne!(matches[0].match_id, matches[1].match_id);
    }

    #[test]
    fn test_market_order() {
        let mut manager = PoolManager::new();
        define_single_leg_pool(&mut manager, 1);

        // Submit passive entry for lineup 0
        manager.create_entry(new_order(1, 0, 6001, 600_000, 600_000)).unwrap();

        // Market order for lineup 1 should calculate portion = 400k
        let (_e, _a, matches, market_eliminations) = manager
            .create_entry(new_market_order(1, 1, 6002, 400_000))
            .unwrap();

        assert_eq!(matches.len(), 1);
        assert_eq!(matches[0].fill_events.len(), 2);

        let market_fill = matches[0].fill_events.iter().find(|f| f.is_aggressor).unwrap();
        assert_eq!(market_fill.matched_portion, 400_000);
        assert_eq!(market_eliminations.len(), 0);
    }

    #[test]
    fn test_market_order_partial_fill() {
        let mut manager = PoolManager::new();
        define_single_leg_pool(&mut manager, 1);

        manager.create_entry(new_order(1, 0, 6101, 600_000, 600_000)).unwrap();

        // Market order with quantity=1_800_000 should only match once
        let (_e, ack, matches, market_eliminations) = manager
            .create_entry(new_market_order(1, 1, 6102, 1_800_000))
            .unwrap();

        assert_eq!(matches.len(), 1);
        assert_eq!(matches[0].matched_quantity, 600_000);
        assert_eq!(market_eliminations.len(), 1);
        assert_eq!(market_eliminations[0].order_id, ack.order_id);
    }

    // --- Cancel tests ---

    #[test]
    fn test_cancel_entry_success() {
        let mut manager = PoolManager::new();
        define_standard_pool(&mut manager, 1);

        let (_e, ack, _m, _me) = manager
            .create_entry(new_order(1, 0, 7001, 250_000, 250_000))
            .unwrap();

        let order_id = ack.order_id;
        assert!(manager.get_slate_id_for_order(order_id).is_some());

        let cancel_ack = manager
            .cancel_entry(CancelOrderBody { order_id })
            .unwrap();

        assert_eq!(cancel_ack.order_id, order_id);
        assert!(manager.get_slate_id_for_order(order_id).is_none());

        // Verify entry is removed from pool
        let pool = manager.get_pool(1).unwrap();
        let state = pool.get_state();
        for book in &state.books {
            assert!(!book.entries.iter().any(|e| e.id == order_id));
        }
    }

    #[test]
    fn test_cancel_entry_not_found() {
        let mut manager = PoolManager::new();

        let result = manager.cancel_entry(CancelOrderBody { order_id: 9999 });
        assert!(result.is_err());
        assert!(result.unwrap_err().contains("not found"));
    }

    #[test]
    fn test_cancel_prevents_fill() {
        let mut manager = PoolManager::new();
        define_standard_pool(&mut manager, 1);

        // Create orders for 3 out of 4 lineups
        let (_e, ack0, _, _me) = manager.create_entry(new_order(1, 0, 8001, 250_000, 250_000)).unwrap();
        let (_e, ack1, _, _me) = manager.create_entry(new_order(1, 1, 8002, 250_000, 250_000)).unwrap();
        let (_e, ack2, _, _me) = manager.create_entry(new_order(1, 2, 8003, 250_000, 250_000)).unwrap();

        // Cancel one
        manager.cancel_entry(CancelOrderBody { order_id: ack1.order_id }).unwrap();

        // 4th order should NOT trigger a match
        let (_e, _a, matches, _me) = manager
            .create_entry(new_order(1, 3, 8004, 250_000, 250_000))
            .unwrap();
        assert_eq!(matches.len(), 0);

        // Verify the remaining orders
        let pool = manager.get_pool(1).unwrap();
        let state = pool.get_state();
        assert!(state.books[0].entries.iter().any(|e| e.id == ack0.order_id));
        assert!(!state.books[1].entries.iter().any(|e| e.id == ack1.order_id));
        assert!(state.books[2].entries.iter().any(|e| e.id == ack2.order_id));
    }

    #[test]
    fn test_cancel_after_partial_fill() {
        let mut manager = PoolManager::new();
        define_standard_pool(&mut manager, 1);

        manager.create_entry(new_order(1, 0, 9001, 250_000, 500_000)).unwrap();
        manager.create_entry(new_order(1, 1, 9002, 250_000, 250_000)).unwrap();
        manager.create_entry(new_order(1, 1, 9003, 250_000, 250_000)).unwrap();
        let (_e, ack2, _, _me) = manager.create_entry(new_order(1, 2, 9004, 250_000, 500_000)).unwrap();

        // First match
        let (_e, _a, matches, _me) = manager
            .create_entry(new_order(1, 3, 9005, 250_000, 250_000))
            .unwrap();
        assert_eq!(matches.len(), 1);

        // Cancel order that still has quantity remaining
        manager.cancel_entry(CancelOrderBody { order_id: ack2.order_id }).unwrap();

        // Second attempt — should not match
        let (_e, _a, matches2, _me) = manager
            .create_entry(new_order(1, 3, 9006, 250_000, 250_000))
            .unwrap();
        assert_eq!(matches2.len(), 0);
    }

    // --- Self-match elimination tests ---

    #[test]
    fn test_eliminations_zero() {
        let mut manager = PoolManager::new();
        define_standard_pool(&mut manager, 1);

        // Different self_match_ids — no eliminations
        manager.create_entry(new_order_with_smid(1, 0, 1001, 250_000, 250_000, 1)).unwrap();
        manager.create_entry(new_order_with_smid(1, 1, 1002, 250_000, 250_000, 2)).unwrap();

        let (eliminations, _a, _m, _me) = manager
            .create_entry(new_order_with_smid(1, 2, 1003, 250_000, 250_000, 3))
            .unwrap();

        assert_eq!(eliminations.len(), 0);
    }

    #[test]
    fn test_eliminations_one() {
        let mut manager = PoolManager::new();
        define_single_leg_pool(&mut manager, 1);

        // Submit entry with self_match_id=42 to lineup 0
        let (_e, ack1, _m, _me) = manager
            .create_entry(new_order_with_smid(1, 0, 2001, 500_000, 250_000, 42))
            .unwrap();

        // Submit entry to lineup 1 with same self_match_id — eliminate lineup 0 entry
        let (eliminations, _a, _m, _me) = manager
            .create_entry(new_order_with_smid(1, 1, 2002, 500_000, 250_000, 42))
            .unwrap();

        assert_eq!(eliminations.len(), 1);
        assert_eq!(eliminations[0].order_id, ack1.order_id);
        assert_eq!(eliminations[0].elimination_description, "Eliminated due to self-match prevention");
    }

    #[test]
    fn test_eliminations_two() {
        let mut manager = PoolManager::new();
        define_single_leg_pool(&mut manager, 1);

        // Two entries in lineup 0 with same self_match_id=77
        let (_e, ack1, _m, _me) = manager
            .create_entry(new_order_with_smid(1, 0, 3001, 500_000, 250_000, 77))
            .unwrap();
        let (_e, ack2, _m, _me) = manager
            .create_entry(new_order_with_smid(1, 0, 3002, 400_000, 200_000, 77))
            .unwrap();

        // Entry in lineup 1 with same self_match_id — both in lineup 0 eliminated
        let (eliminations, _a, _m, _me) = manager
            .create_entry(new_order_with_smid(1, 1, 3003, 500_000, 250_000, 77))
            .unwrap();

        assert_eq!(eliminations.len(), 2);
        let eliminated_ids: Vec<u64> = eliminations.iter().map(|e| e.order_id).collect();
        assert!(eliminated_ids.contains(&ack1.order_id));
        assert!(eliminated_ids.contains(&ack2.order_id));
    }
}
