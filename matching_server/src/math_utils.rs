///! Utility for finding the **highest common denominator** (GCD)
///! among a list of integers. Used to determine the smallest step size
///! all pool participants can share, guaranteeing integer precision.
///!
///! This version assumes input is always valid (non-empty, positive),
///! and uses `assert!()` for invariants rather than Option or Result.

/// Compute the GCD (greatest common divisor) of two positive integers.
/// Uses the classic Euclidean algorithm.
fn gcd(a: u64, b: u64) -> u64 {
    assert!(a > 0 && b > 0, "Inputs to gcd() must be positive");
    let mut x = a;
    let mut y = b;
    while y != 0 {
        let temp = y;
        y = x % y;
        x = temp;
    }
    x
}

/// Compute the **highest common denominator** (GCD)
/// across all numbers in the list.
///
/// # Panics
/// - If `numbers` is empty
/// - If any element ≤ 0
///
/// # Example
/// ```
/// let nums = vec![10, 20, 30];
/// let gcd = crate::matching_server::math_utils::highest_common_denominator(&nums);
/// assert_eq!(gcd, 10);
/// ```
pub fn highest_common_denominator(numbers: &[u64]) -> u64 {
    assert!(!numbers.is_empty(), "numbers cannot be empty");
    assert!(
        numbers.iter().all(|&n| n > 0),
        "all numbers must be positive"
    );

    let mut result = numbers[0];
    for &num in &numbers[1..] {
        result = gcd(result, num);
        if result == 1 {
            // No need to continue once it reaches 1
            break;
        }
    }
    result
}

#[cfg(test)]
mod tests {
    use super::*;

    /// Basic sanity: shared common factor
    #[test]
    fn test_basic_common_factor() {
        let nums = vec![10, 20, 30];
        let result = highest_common_denominator(&nums);
        assert_eq!(result, 10);
    }

    /// Numbers with no common factor other than 1
    #[test]
    fn test_no_common_factor() {
        let nums = vec![7, 11, 13];
        let result = highest_common_denominator(&nums);
        assert_eq!(result, 1);
    }

    /// Handles large but related numbers
    #[test]
    fn test_large_numbers() {
        let nums = vec![1200, 1800, 6000];
        let result = highest_common_denominator(&nums);
        assert_eq!(result, 600);
    }

    /// Single element — GCD is the number itself
    #[test]
    fn test_single_element() {
        let nums = vec![25];
        let result = highest_common_denominator(&nums);
        assert_eq!(result, 25);
    }

    /// Verify that empty input panics
    #[test]
    #[should_panic(expected = "numbers cannot be empty")]
    fn test_empty_panics() {
        let nums: Vec<u64> = vec![];
        let _ = highest_common_denominator(&nums);
    }

    /// Verify that zero or negative numbers panic
    #[test]
    #[should_panic(expected = "all numbers must be positive")]
    fn test_nonpositive_panics() {
        let nums = vec![10, 0, 20];
        let _ = highest_common_denominator(&nums);
    }
}
