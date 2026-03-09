// PoolUtils.rs
// Purely functional integer-only pool math that guarantees no leftover funds.
//
// All functions are stateless — they only depend on inputs and return values.
// No floats, no rounding, no persistent state.

/// Compute the sum of all participants' units.
pub fn sum_units(participants: &[u64]) -> u64 {
    participants.iter().sum()
}

/// Compute how many units remain in the pool.
///
/// # Arguments
/// * `participants` - List of participants' assigned units
/// * `total_units` - The total capacity of the pool
///
/// # Returns
/// The number of remaining units (0 if overfilled)
pub fn calculate_remaining_units(participants: &[u64], total_units: u64) -> u64 {
    let used = sum_units(participants);
    total_units.saturating_sub(used)
}

/// Returns a new list including the new participant's contribution.
///
/// This does not mutate the original list.
pub fn extend_with_new(participants: &[u64], new_units: u64) -> Vec<u64> {
    let mut v = participants.to_vec();
    v.push(new_units);
    v
}

/// Checks whether the pool is exactly full (no leftover units).
pub fn is_full(participants: &[u64], total_units: u64) -> bool {
    sum_units(participants) == total_units
}

/// Represents a leg with its security ID and over/under status
#[derive(Clone, Debug, PartialEq, Eq)]
pub struct Leg {
    pub leg_security_id: u128,
    pub is_over: bool,
}

/// Converts a proto UUID to a u128 for internal use.
pub fn uuid_to_u128(uuid: &crate::common::Uuid) -> u128 {
    ((uuid.upper as u128) << 64) | (uuid.lower as u128)
}

/// Creates a canonical pool key from a list of leg security IDs.
/// The key is the sorted list of security IDs, ensuring that different
/// orderings of the same securities map to the same pool.
pub fn create_pool_key(leg_security_ids: &[u128]) -> Vec<u128> {
    let mut sorted = leg_security_ids.to_vec();
    sorted.sort_unstable();
    sorted
}

/// Calculates the lineup index from a list of legs.
/// The lineup index is calculated based on the isOver values in canonical order
/// (sorted by leg_security_id).
///
/// # Formula
/// For legs sorted by leg_security_id:
/// lineup_index = sum(is_over[i] * 2^i for i in 0..n)
///
/// # Example
/// legs = [{legSecurityId: 102, isOver: true}, {legSecurityId: 101, isOver: false}]
/// Sorted by ID: [{legSecurityId: 101, isOver: false}, {legSecurityId: 102, isOver: true}]
/// lineup_index = false * 2^0 + true * 2^1 = 0 + 2 = 2
pub fn calculate_lineup_index(legs: &[Leg]) -> u64 {
    // Create a sorted copy of the legs by leg_security_id
    let mut sorted_legs = legs.to_vec();
    sorted_legs.sort_by_key(|leg| leg.leg_security_id);

    // Calculate lineup index using binary representation
    let mut lineup_index: u64 = 0;
    for (i, leg) in sorted_legs.iter().enumerate() {
        if leg.is_over {
            lineup_index |= 1 << i;
        }
    }

    lineup_index
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_basic_fill() {
        let total = 1000;
        let participants = vec![400, 500];
        let rem = calculate_remaining_units(&participants, total);
        assert_eq!(rem, 100); // 400 + 500 + 100 = 1000
        let all = extend_with_new(&participants, rem);
        assert!(is_full(&all, total));
    }

    #[test]
    fn test_overfilled_case() {
        let total = 1000;
        let participants = vec![700, 400]; // total = 1100
        let rem = calculate_remaining_units(&participants, total);
        assert_eq!(rem, 0);
        assert!(!is_full(&participants, total));
    }

    #[test]
    fn test_multiple_small_additions() {
        let total = 10;
        let mut participants = vec![];
        for _ in 0..9 {
            let rem = calculate_remaining_units(&participants, total);
            participants = extend_with_new(&participants, 1);
            assert!(rem >= 1);
        }
        assert_eq!(calculate_remaining_units(&participants, total), 1);
        let final_participants = extend_with_new(&participants, 1);
        assert!(is_full(&final_participants, total));
    }

    #[test]
    fn test_exact_full_pool() {
        let total = 100;
        let participants = vec![40, 30, 30];
        assert!(is_full(&participants, total));
        assert_eq!(calculate_remaining_units(&participants, total), 0);
    }

    #[test]
    fn test_empty_pool() {
        let total = 100;
        let participants: Vec<u64> = vec![];
        assert_eq!(calculate_remaining_units(&participants, total), 100);
        assert!(!is_full(&participants, total));
    }

    #[test]
    fn test_create_pool_key() {
        let key1 = create_pool_key(&[101u128, 102, 103]);
        let key2 = create_pool_key(&[103u128, 101, 102]);
        let key3 = create_pool_key(&[102u128, 103, 101]);

        assert_eq!(key1, vec![101u128, 102, 103]);
        assert_eq!(key2, vec![101u128, 102, 103]);
        assert_eq!(key3, vec![101u128, 102, 103]);
        assert_eq!(key1, key2);
        assert_eq!(key2, key3);
    }

    #[test]
    fn test_calculate_lineup_index_simple() {
        // Two legs: [101, 102] with isOver [false, true]
        let legs = vec![
            Leg { leg_security_id: 101, is_over: false },
            Leg { leg_security_id: 102, is_over: true },
        ];
        assert_eq!(calculate_lineup_index(&legs), 2); // 0 * 2^0 + 1 * 2^1 = 2
    }

    #[test]
    fn test_calculate_lineup_index_order_independent() {
        // Same legs, different order
        let legs1 = vec![
            Leg { leg_security_id: 101, is_over: false },
            Leg { leg_security_id: 102, is_over: true },
        ];
        let legs2 = vec![
            Leg { leg_security_id: 102, is_over: true },
            Leg { leg_security_id: 101, is_over: false },
        ];

        assert_eq!(calculate_lineup_index(&legs1), calculate_lineup_index(&legs2));
        assert_eq!(calculate_lineup_index(&legs1), 2);
    }

    #[test]
    fn test_calculate_lineup_index_three_legs() {
        // Three legs: [101, 102, 103] with isOver [true, false, true]
        let legs = vec![
            Leg { leg_security_id: 101, is_over: true },
            Leg { leg_security_id: 102, is_over: false },
            Leg { leg_security_id: 103, is_over: true },
        ];
        // 1 * 2^0 + 0 * 2^1 + 1 * 2^2 = 1 + 0 + 4 = 5
        assert_eq!(calculate_lineup_index(&legs), 5);
    }

    #[test]
    fn test_calculate_lineup_index_three_legs_reordered() {
        // Same as above but in different order
        let legs = vec![
            Leg { leg_security_id: 103, is_over: true },
            Leg { leg_security_id: 101, is_over: true },
            Leg { leg_security_id: 102, is_over: false },
        ];
        assert_eq!(calculate_lineup_index(&legs), 5);
    }

    #[test]
    fn test_calculate_lineup_index_all_over() {
        let legs = vec![
            Leg { leg_security_id: 101, is_over: true },
            Leg { leg_security_id: 102, is_over: true },
        ];
        // 1 * 2^0 + 1 * 2^1 = 1 + 2 = 3
        assert_eq!(calculate_lineup_index(&legs), 3);
    }

    #[test]
    fn test_calculate_lineup_index_all_under() {
        let legs = vec![
            Leg { leg_security_id: 101, is_over: false },
            Leg { leg_security_id: 102, is_over: false },
        ];
        // 0 * 2^0 + 0 * 2^1 = 0
        assert_eq!(calculate_lineup_index(&legs), 0);
    }
}
