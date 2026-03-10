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

/// Converts a proto UUID to a u128 for internal use.
pub fn uuid_to_u128(uuid: &crate::common::Uuid) -> u128 {
    ((uuid.upper as u128) << 64) | (uuid.lower as u128)
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
}
