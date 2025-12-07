// entry_pool.rs
//
// EntryPool manages a multi-sided pool where entries are submitted for different lineups.
// Similar to an exchange order book, but instead of buy/sell sides, there are 2^N different
// lineup books (one for each possible combination of outcomes in an N-leg slate).
//
// Key Concepts:
// - A "slate" has N legs (e.g., player stat predictions)
// - Each leg has 2 outcomes (over/under), creating 2^N possible lineups
// - Users submit entries backing a specific lineup with a portion of the pool and quantity
// - When the best entry from each lineup can form a valid pool, a "match" occurs
// - In one match, there are 2^N fills (one from each entry in each lineup)
// - Fills consume quantity from matched entries

use std::fmt;

/// Unique identifier for each entry in the pool
pub type EntryId = u64;

/// Type of entry being submitted
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum EntryType {
    /// Limit entry: user specifies exact portion size
    Limit,
    /// Market entry: portion is calculated to complete the pool
    Market,
}

/// Parameters for submitting an entry
#[derive(Debug, Clone, Copy)]
pub struct EntryParameters {
    /// unique ID for the entry
    pub entry_id: u64,
    /// Type of entry (Limit or Market)
    pub entry_type: EntryType,
    /// Portion of the pool this entry backs (in units)
    /// For Market entries, this value is ignored and calculated automatically
    pub portion: u64,
    /// Number of times this entry can match (how many portions to provide)
    pub quantity: u64,
    /// Optional self-match prevention ID. If Some(id), entries in other lineups with the same
    /// self_match_id will be cancelled when this entry is submitted. Entries in the same lineup
    /// with the same self_match_id are allowed to coexist.
    pub self_match_id: Option<u64>,
}

/// Represents a single entry in a lineup's book
#[derive(Debug, Clone)]
pub struct Entry {
    /// Unique identifier for this entry
    pub id: EntryId,
    /// Which lineup (0 to 2^N - 1) this entry is backing
    pub lineup_index: usize,
    /// Type of entry (Limit or Market)
    pub entry_type: EntryType,
    /// Portion of the pool this entry backs (in units)
    pub portion: u64,
    /// Number of times this entry can match (how many portions to provide)
    pub quantity: u64,
    /// Timestamp/sequence for FIFO ordering among same-portion entries
    pub sequence: u64,
    /// Optional self-match prevention ID
    pub self_match_id: Option<u64>,
}

impl Entry {
    /// Returns true if this entry has enough quantity remaining to stay in the book
    pub fn has_remaining_quantity(&self) -> bool {
        self.quantity > 0
    }
    
    /// Reduces the quantity by the given amount
    pub fn consume_quantity(&mut self, amount: u64) {
        self.quantity = self.quantity.saturating_sub(amount);
    }
}

/// Book state for a single lineup - contains all entries backing this lineup
#[derive(Debug, Clone)]
pub struct BookState {
    /// All entries in this book, maintained in priority order:
    /// 1. Largest portion first
    /// 2. If portions are equal, earliest sequence (FIFO)
    pub entries: Vec<Entry>,
}

impl BookState {
    /// Creates a new empty book
    pub fn new() -> Self {
        BookState {
            entries: Vec::new(),
        }
    }
    
    /// Adds an entry to the book and maintains sort order
    /// (largest portion first, then FIFO by sequence)
    pub fn add_entry(&mut self, entry: Entry) {
        self.entries.push(entry);
        // Sort by portion descending, then by sequence ascending (FIFO)
        self.entries.sort_by(|a, b| {
            match b.portion.cmp(&a.portion) {
                std::cmp::Ordering::Equal => a.sequence.cmp(&b.sequence),
                other => other,
            }
        });
    }
    
    /// Returns the best (highest priority) entry in this book, if any
    pub fn best_entry(&self) -> Option<&Entry> {
        self.entries.first()
    }
    
    /// Returns a mutable reference to the best entry, if any
    pub fn best_entry_mut(&mut self) -> Option<&mut Entry> {
        self.entries.first_mut()
    }
    
    /// Removes entries that no longer have sufficient quantity to match
    pub fn remove_depleted_entries(&mut self) {
        self.entries.retain(|entry| entry.has_remaining_quantity());
    }
    
    /// Removes all market entries (i.e., entries that should not rest)
    /// Returns the IDs of entries that were removed
    pub fn remove_market_entries(&mut self) -> Vec<u64> {
        let removed_ids: Vec<u64> = self.entries
            .iter()
            .filter(|entry| entry.entry_type == EntryType::Market)
            .map(|entry| entry.id)
            .collect();

        self.entries.retain(|entry| entry.entry_type != EntryType::Market);

        removed_ids
    }

    /// Finds and returns a mutable reference to an entry by ID
    pub fn find_entry_mut(&mut self, id: EntryId) -> Option<&mut Entry> {
        self.entries.iter_mut().find(|e| e.id == id)
    }
}

impl fmt::Display for BookState {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        if self.entries.is_empty() {
            write!(f, "    [Empty Book]")?;
        } else {
            for (idx, entry) in self.entries.iter().enumerate() {
                write!(
                    f,
                    "    [{}/{}] ID:{} portion:{} qty:{} seq:{}",
                    idx,
                    self.entries.len(),
                    entry.id,
                    entry.portion,
                    entry.quantity,
                    entry.sequence
                )?;
                if idx < self.entries.len() - 1 {
                    writeln!(f)?;
                }
            }
        }
        Ok(())
    }
}

/// Complete state of the pool across all lineups
#[derive(Debug, Clone)]
pub struct PoolState {
    /// Total units in the pool (e.g., 1000)
    pub total_units: u64,
    /// Number of legs in the slate
    pub num_legs: usize,
    /// Book for each lineup (2^num_legs books total)
    pub books: Vec<BookState>,
}

impl PoolState {
    /// Creates a new pool state with empty books for each lineup
    pub fn new(total_units: u64, num_legs: usize) -> Self {
        let num_lineups = 1 << num_legs; // 2^num_legs
        let mut books = Vec::with_capacity(num_lineups);
        for _ in 0..num_lineups {
            books.push(BookState::new());
        }
        
        PoolState {
            total_units,
            num_legs,
            books,
        }
    }
    
    /// Returns the number of lineups (2^num_legs)
    pub fn num_lineups(&self) -> usize {
        self.books.len()
    }
}

impl fmt::Display for PoolState {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        writeln!(f, "PoolState:")?;
        writeln!(f, "  Total Units: {}", self.total_units)?;
        writeln!(f, "  Legs: {} (Lineups: {})", self.num_legs, self.num_lineups())?;
        writeln!(f, "  Books:")?;
        for (lineup_idx, book) in self.books.iter().enumerate() {
            writeln!(f, "    Lineup {}:", lineup_idx)?;
            writeln!(f, "{}", book)?;
        }
        Ok(())
    }
}

/// Represents a single fill in a match, including original and matched portions
#[derive(Debug, Clone)]
pub struct FilledEntry {
    /// The entry that was matched
    pub entry: Entry,
    /// Original portion before any aggressor adjustment
    pub original_portion: u64,
    /// matched portion after aggressor adjustment (may differ from original for aggressor)
    pub matched_portion: u64,
    /// True only if this is the final fill for this entry (entry will be removed from book)
    pub is_complete: bool,
}

/// Represents a successful match across all lineups
#[derive(Debug, Clone)]
pub struct MatchInfo {
    /// One filled entry from each lineup (2^N entries total)
    pub filled_entries: Vec<FilledEntry>,
    /// Index of the lineup that was the aggressor
    pub aggressor_lineup_index: usize,
    /// Quantity that was matched in this match (same for all entries)
    pub matched_quantity: u64,
}

impl fmt::Display for MatchInfo {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        writeln!(f, "MatchInfo (Aggressor: Lineup {}, Matched Qty: {}):",
                 self.aggressor_lineup_index, self.matched_quantity)?;
        for fill in &self.filled_entries {
            let aggressor_marker = if fill.entry.lineup_index == self.aggressor_lineup_index {
                " [AGGRESSOR]"
            } else {
                ""
            };
            writeln!(
                f,
                "  Lineup {}: ID:{} portion:{}→{} {}",
                fill.entry.lineup_index,
                fill.entry.id,
                fill.original_portion,
                fill.matched_portion,
                aggressor_marker
            )?;
        }
        Ok(())
    }
}

/// Information returned when an entry is successfully submitted
#[derive(Debug, Clone)]
pub struct SubmitInfo {
    /// Entry IDs that were cancelled due to self_match_id conflicts
    pub cancelled_entry_ids: Vec<u64>,
    /// Any matches that occurred as a result of this submission
    pub match_infos: Vec<MatchInfo>,
    /// Market entries that were removed from the book because they weren't fully filled. In practice, there will only be zero or one of these.
    pub cancelled_market_entry_ids: Vec<u64>,
}

/// Result of submitting an entry - either success with SubmitInfo or error message
pub type SubmitResult = Result<SubmitInfo, String>;

/// The main EntryPool struct that manages all lineups and matching logic
pub struct EntryPool {
    /// Current state of all books
    state: PoolState,
    /// Counter for sequence numbers (for FIFO ordering)
    next_sequence: u64,
}

impl EntryPool {
    /// Creates a new EntryPool
    ///
    /// # Arguments
    /// * `total_units` - Total capacity of the pool (e.g., 1000)
    /// * `num_legs` - Number of legs in the slate (creates 2^num_legs lineups)
    pub fn new(total_units: u64, num_legs: usize) -> Self {
        EntryPool {
            state: PoolState::new(total_units, num_legs),
            next_sequence: 0,
        }
    }
    
    /// Returns a reference to the current pool state
    pub fn get_state(&self) -> &PoolState {
        &self.state
    }
    
    /// Submits a new entry to the pool
    ///
    /// # Arguments
    /// * `lineup_index` - Which lineup (0 to 2^N - 1) this entry backs
    /// * `params` - Entry parameters (type, portion, quantity)
    ///
    /// # Returns
    /// * `Ok(SubmitInfo)` - Contains any matches that occurred and completed entry IDs
    /// * `Err(String)` - Error message if submission failed
    ///
    /// For Market entries: attempts immediate matching. If no valid match exists,
    /// the market entry is rejected (not rested on book).
    ///
    /// For Limit entries: added to book, then checked for potential matches.
    /// May trigger multiple matches if quantity is large enough.
    pub fn submit_entry(
        &mut self,
        lineup_index: usize,
        params: EntryParameters,
    ) -> SubmitResult {
        // Validate lineup index
        if lineup_index >= self.state.num_lineups() {
            return Err(format!(
                "Invalid lineup_index: {} (must be 0 to {})",
                lineup_index,
                self.state.num_lineups() - 1
            ));
        }
        
        // Validate quantity > 0
        if params.quantity == 0 {
            return Err("Quantity must be greater than 0".to_string());
        }
        
        // Calculate portion based on entry type
        let portion = match params.entry_type {
            EntryType::Limit => {
                // For limit entries, use provided portion and validate it's > 0
                if params.portion == 0 {
                    return Err("Portion must be greater than 0".to_string());
                }
                params.portion
            }
            EntryType::Market => {
                // For market entries, calculate the portion needed to complete the pool
                // If other lineups already sum to >= total_units, we'll use portion=1
                // and let the fill logic reduce passive entries as needed
                let other_lineups: Vec<usize> = (0..self.state.num_lineups())
                    .filter(|&i| i != lineup_index)
                    .collect();
                
                let mut other_sum = 0u64;
                for &other_idx in &other_lineups {
                    if let Some(entry) = self.state.books[other_idx].best_entry() {
                        other_sum += entry.portion;
                    } else {
                        // Can't form a valid fill - missing entries from some lineups
                        return Err("Market entry rejected: insufficient passive entries for fill".to_string());
                    }
                }
                
                if other_sum >= self.state.total_units {
                    // Other lineups already fill or overfill the pool
                    // Set portion to 1 and let fill logic reduce passive entries
                    1
                } else {
                    self.state.total_units - other_sum
                }
            }
        };
        
        // Create the entry
        let entry = Entry {
            id: params.entry_id,
            lineup_index,
            entry_type: params.entry_type,
            portion,
            quantity: params.quantity,
            sequence: self.next_sequence,
            self_match_id: params.self_match_id,
        };
        self.next_sequence += 1;

        // Handle self-match prevention: cancel entries in OTHER lineups with same self_match_id
        // It is important that all error cases are checked prior to cancelling orders so that the order book is unchanged in the event of an error.
        let mut cancelled_entry_ids = Vec::new();
        if let Some(self_match_id) = params.self_match_id {
            for (other_lineup_idx, book) in self.state.books.iter_mut().enumerate() {
                // Only cancel entries in different lineups
                if other_lineup_idx != lineup_index {
                    let entries_to_cancel: Vec<u64> = book.entries.iter()
                        .filter(|e| e.self_match_id == Some(self_match_id))
                        .map(|e| e.id)
                        .collect();

                    for entry_id in entries_to_cancel {
                        book.entries.retain(|e| e.id != entry_id);
                        cancelled_entry_ids.push(entry_id);
                    }
                }
            }
        }

        // Add to book
        self.state.books[lineup_index].add_entry(entry);

        // Attempt matches
        let mut match_infos = self.attempt_matches();
        
        // Collect completed entry IDs
        let mut completed_entry_ids = Vec::new();
        for match_info in &match_infos {
            for filled_entry in &match_info.filled_entries {
                let lineup_idx = filled_entry.entry.lineup_index;
                let entry_id = filled_entry.entry.id;

                // no need to add the same completed entry_id twice if it was part of multiple matches
                if completed_entry_ids.contains(&entry_id) {
                    continue;
                }

                let still_on_book = self.state.books[lineup_idx]
                    .entries
                    .iter()
                    .any(|e| e.id == entry_id);
                if !still_on_book {
                    completed_entry_ids.push(entry_id);
                }
            }
        }

        // Update is_complete flag for entries in their final match
        for entry_id in &completed_entry_ids {
            // Find the last match index where this entry appears
            let mut last_match_idx = None;
            for (match_idx, match_info) in match_infos.iter().enumerate() {
                if match_info.filled_entries.iter().any(|fe| fe.entry.id == *entry_id) {
                    last_match_idx = Some(match_idx);
                }
            }

            // Mark is_complete = true for this entry in its last match
            if let Some(match_idx) = last_match_idx {
                for filled_entry in &mut match_infos[match_idx].filled_entries {
                    if filled_entry.entry.id == *entry_id {
                        filled_entry.is_complete = true;
                    }
                }
            }
        }

        // Remove market entries from all books
        let mut cancelled_market_entry_ids = Vec::new();
        for book in &mut self.state.books {
            cancelled_market_entry_ids.extend(book.remove_market_entries());
        }

        Ok(SubmitInfo {
            cancelled_entry_ids,
            match_infos,
            cancelled_market_entry_ids,
        })
    }
    
    /// Cancels an entry by removing it from the pool
    ///
    /// # Arguments
    /// * `entry_id` - The ID of the entry to cancel
    ///
    /// # Returns
    /// * `Ok(())` - Entry was successfully cancelled
    /// * `Err(String)` - Error message if entry was not found
    pub fn cancel_entry(&mut self, entry_id: u64) -> Result<(), String> {
        // Search through all books to find and remove the entry
        for book in &mut self.state.books {
            let initial_len = book.entries.len();
            book.entries.retain(|entry| entry.id != entry_id);

            // If we removed an entry, return success
            if book.entries.len() < initial_len {
                return Ok(());
            }
        }

        // Entry not found in any book
        Err(format!("Entry with ID {} not found", entry_id))
    }

    /// Attempts to create matches from the current book state
    /// Continues attempting matches until no valid match can be made
    fn attempt_matches(&mut self) -> Vec<MatchInfo> {
        let mut match_infos = Vec::new();

        loop {
            match self.try_create_match() {
                Some(match_info) => match_infos.push(match_info),
                None => break,
            }
        }

        match_infos
    }

    /// Attempts to create a single match from best entries across all lineups
    /// Returns Some(MatchInfo) if successful, None if no valid match exists
    fn try_create_match(&mut self) -> Option<MatchInfo> {
        // Collect best entry from each lineup
        let mut best_entries = Vec::new();
        for (lineup_idx, book) in self.state.books.iter().enumerate() {
            if let Some(entry) = book.best_entry() {
                best_entries.push((lineup_idx, entry.clone()));
            } else {
                // Missing entry from at least one lineup - can't fill
                return None;
            }
        }
        
        // Determine which entry is the aggressor (most recent = highest sequence)
        let (aggressor_idx, aggressor_lineup_index) = best_entries
            .iter()
            .enumerate()
            .max_by_key(|(_, (_, entry))| entry.sequence)
            .map(|(idx, (lineup_idx, _))| (idx, *lineup_idx))?;
        
        // Calculate initial total portion
        let total_portion: u64 = best_entries.iter().map(|(_, e)| e.portion).sum();
        
        // Calculate adjusted portions for this fill
        let mut adjusted_portions: Vec<u64> = best_entries.iter().map(|(_, e)| e.portion).collect();
        
        if total_portion > self.state.total_units {
            let excess = total_portion - self.state.total_units;
            
            // First, try to reduce aggressor as much as possible (down to 1)
            let max_aggressor_reduction = adjusted_portions[aggressor_idx].saturating_sub(1);
            let aggressor_reduction = excess.min(max_aggressor_reduction);
            adjusted_portions[aggressor_idx] -= aggressor_reduction;
            
            let remaining_excess = excess - aggressor_reduction;
            
            if remaining_excess > 0 {
                // Need to reduce passive entries from oldest to newest sequence
                let mut passive_entries_by_sequence: Vec<(usize, u64)> = best_entries
                    .iter()
                    .enumerate()
                    .filter(|(idx, _)| *idx != aggressor_idx)
                    .map(|(idx, (_, entry))| (idx, entry.sequence))
                    .collect();
                passive_entries_by_sequence.sort_by_key(|(_, seq)| *seq);
                
                let mut remaining = remaining_excess;
                for (idx, _) in passive_entries_by_sequence {
                    if remaining == 0 {
                        break;
                    }
                    let max_reduction = adjusted_portions[idx].saturating_sub(1);
                    let reduction = remaining.min(max_reduction);
                    adjusted_portions[idx] -= reduction;
                    remaining -= reduction;
                }
                
                // If we still can't reduce enough, no fill possible
                if remaining > 0 {
                    return None;
                }
            }
        } else if total_portion < self.state.total_units {
            // Not enough to fill the pool
            return None;
        }
        
        // Verify all portions are > 0
        if adjusted_portions.iter().any(|&p| p == 0) {
            return None;
        }
        
        // Calculate minimum quantity across all entries
        let min_quantity = best_entries.iter().map(|(_, e)| e.quantity).min()?;
        
        if min_quantity == 0 {
            return None;
        }
        
        // Create filled entries
        let mut filled_entries = Vec::new();
        for (idx, (_lineup_idx, entry)) in best_entries.iter().enumerate() {
            filled_entries.push(FilledEntry {
                entry: entry.clone(),
                original_portion: entry.portion,
                matched_portion: adjusted_portions[idx],
                is_complete: false, // Will be set properly in submit_entry after all matches
            });
        }

        // Apply the fills - consume quantity from all matched entries
        for fill in &filled_entries {
            if let Some(book_entry) = self.state.books[fill.entry.lineup_index]
                .find_entry_mut(fill.entry.id)
            {
                book_entry.consume_quantity(min_quantity);
            }
        }

        // Remove depleted entries from all books
        for book in &mut self.state.books {
            book.remove_depleted_entries();
        }

        Some(MatchInfo {
            filled_entries,
            aggressor_lineup_index,
            matched_quantity: min_quantity,
        })
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    
    #[test]
    fn test_basic_limit_entry() {
        let mut pool = EntryPool::new(1000, 2); // 4 lineups

        // Submit entries to all 4 lineups (quantity=1 means willing to provide portion once)
        let submit0 = pool.submit_entry(0, EntryParameters {
            entry_id: 0,
            entry_type: EntryType::Limit,
            portion: 250,
            quantity: 1,
            self_match_id: None,
        }).unwrap();
        let submit1 = pool.submit_entry(1, EntryParameters {
            entry_id: 1,
            entry_type: EntryType::Limit,
            portion: 250,
            quantity: 1,
            self_match_id: None,
        }).unwrap();
        let submit2 = pool.submit_entry(2, EntryParameters {
            entry_id: 2,
            entry_type: EntryType::Limit,
            portion: 250,
            quantity: 1,
            self_match_id: None,
        }).unwrap();
        let submit3 = pool.submit_entry(3, EntryParameters {
            entry_id: 3,
            entry_type: EntryType::Limit,
            portion: 250,
            quantity: 1,
            self_match_id: None,
        }).unwrap();
        
        // Last submission should trigger a match
        assert_eq!(submit0.match_infos.len(), 0);
        assert_eq!(submit1.match_infos.len(), 0);
        assert_eq!(submit2.match_infos.len(), 0);
        assert_eq!(submit3.match_infos.len(), 1);

        // No market entries, so cancelled_market_entry_ids should be empty
        assert_eq!(submit3.cancelled_market_entry_ids.len(), 0);

        // All entries should be consumed in the match
        let state = pool.get_state();
        for book in &state.books {
            assert_eq!(book.entries.len(), 0);
        }
    }
    
    #[test]
    fn test_aggressor_portion_reduction() {
        let mut pool = EntryPool::new(1000, 2); // 4 lineups
        
        // Submit passive entries
        pool.submit_entry(0, EntryParameters {
            entry_id: 0,
            entry_type: EntryType::Limit,
            portion: 300,
            quantity: 1,
            self_match_id: None,
        }).unwrap();
        pool.submit_entry(1, EntryParameters {
            entry_id: 1,
            entry_type: EntryType::Limit,
            portion: 300,
            quantity: 1,
            self_match_id: None,
        }).unwrap();
        pool.submit_entry(2, EntryParameters {
            entry_id: 2,
            entry_type: EntryType::Limit,
            portion: 300,
            quantity: 1,
            self_match_id: None,
        }).unwrap();

        // Aggressor with portion that would overfill
        let submit = pool.submit_entry(3, EntryParameters {
            entry_id: 3,
            entry_type: EntryType::Limit,
            portion: 200,
            quantity: 1,
            self_match_id: None,
        }).unwrap();
        
        // Should have 1 match
        assert_eq!(submit.match_infos.len(), 1);
        
        // Aggressor should have portion reduced from 200 to 100
        let match_info =&submit.match_infos[0];
        let aggressor_entry = match_info.filled_entries.iter()
            .find(|e| e.entry.lineup_index == 3)
            .unwrap();
        assert_eq!(aggressor_entry.original_portion, 200);
        assert_eq!(aggressor_entry.matched_portion, 100);
        assert_eq!(match_info.matched_quantity, 1);
        
        // All entries consumed
        let state = pool.get_state();
        for book in &state.books {
            assert_eq!(book.entries.len(), 0);
        }
    }
    
    #[test]
    fn test_market_entry_success() {
        let mut pool = EntryPool::new(1000, 2); // 4 lineups
        
        // Submit passive entries totaling 700
        pool.submit_entry(0, EntryParameters {
            entry_id: 0,
            entry_type: EntryType::Limit,
            portion: 200,
            quantity: 1,
            self_match_id: None,
        }).unwrap();
        pool.submit_entry(1, EntryParameters {
            entry_id: 1,
            entry_type: EntryType::Limit,
            portion: 250,
            quantity: 1,
            self_match_id: None,
        }).unwrap();
        pool.submit_entry(2, EntryParameters {
            entry_id: 2,
            entry_type: EntryType::Limit,
            portion: 250,
            quantity: 1,
            self_match_id: None,
        }).unwrap();

        // Market entry should calculate portion = 300
        let submit = pool.submit_entry(3, EntryParameters {
            entry_id: 3,
            entry_type: EntryType::Market,
            portion: 0, // ignored for market entries
            quantity: 1,
            self_match_id: None,
        }).unwrap();
        
        // Should have 1 match
        assert_eq!(submit.match_infos.len(), 1);
        let match_info =&submit.match_infos[0];
        assert_eq!(match_info.filled_entries[match_info.aggressor_lineup_index].matched_portion, 300);
        assert_eq!(match_info.matched_quantity, 1);

        // Market entry was fully filled, so it should not be in cancelled_market_entry_ids
        assert_eq!(submit.cancelled_market_entry_ids.len(), 0);

        // All consumed
        let state = pool.get_state();
        for book in &state.books {
            assert_eq!(book.entries.len(), 0);
        }
    }
    
    #[test]
    fn test_market_entry_rejection() {
        let mut pool = EntryPool::new(1000, 2); // 4 lineups
        
        // Only 2 lineups have entries
        pool.submit_entry(0, EntryParameters {
            entry_id: 0,
            entry_type: EntryType::Limit,
            portion: 500,
            quantity: 1,
            self_match_id: None,
        }).unwrap();
        pool.submit_entry(1, EntryParameters {
            entry_id: 1,
            entry_type: EntryType::Limit,
            portion: 500,
            quantity: 1,
            self_match_id: None,
        }).unwrap();

        // Market entry should be rejected - missing lineups
        let result = pool.submit_entry(2, EntryParameters {
            entry_id: 2,
            entry_type: EntryType::Market,
            portion: 0,
            quantity: 1,
            self_match_id: None,
        });
        assert!(result.is_err());
    }
    
    #[test]
    fn test_multiple_fill_events_from_one_aggressor() {
        let mut pool = EntryPool::new(1000, 2); // 4 lineups
        
        // Submit passive entries with quantity=2 (can match 2 times each)
        pool.submit_entry(0, EntryParameters {
            entry_id: 0,
            entry_type: EntryType::Limit,
            portion: 250,
            quantity: 2,
            self_match_id: None,
        }).unwrap();
        pool.submit_entry(1, EntryParameters {
            entry_id: 1,
            entry_type: EntryType::Limit,
            portion: 250,
            quantity: 2,
            self_match_id: None,
        }).unwrap();
        pool.submit_entry(2, EntryParameters {
            entry_id: 2,
            entry_type: EntryType::Limit,
            portion: 250,
            quantity: 1,
            self_match_id: None,
        }).unwrap();
        pool.submit_entry(2, EntryParameters {
            entry_id: 3,
            entry_type: EntryType::Limit,
            portion: 250,
            quantity: 1,
            self_match_id: None,
        }).unwrap();

        // Aggressor with quantity=2 (can match 2 times)
        let submit = pool.submit_entry(3, EntryParameters {
            entry_id: 4,
            entry_type: EntryType::Limit,
            portion: 250,
            quantity: 2,
            self_match_id: None,
        }).unwrap();

        // Should have 2 matches
        assert_eq!(submit.match_infos.len(), 2);
        
        // All should be consumed in 2 matches
        let state = pool.get_state();
        for book in &state.books {
            assert_eq!(book.entries.len(), 0);
        }
    }
    
    #[test]
    fn test_priority_ordering() {
        let mut pool = EntryPool::new(1000, 1); // 2 lineups
        
        // Submit entries with different portions to lineup 0
        pool.submit_entry(0, EntryParameters {
            entry_id: 0,
            entry_type: EntryType::Limit,
            portion: 300,
            quantity: 1,
            self_match_id: None,
        }).unwrap();
        pool.submit_entry(0, EntryParameters {
            entry_id: 1,
            entry_type: EntryType::Limit,
            portion: 500,
            quantity: 1,
            self_match_id: None,
        }).unwrap(); // Largest
        pool.submit_entry(0, EntryParameters {
            entry_id: 2,
            entry_type: EntryType::Limit,
            portion: 400,
            quantity: 1,
            self_match_id: None,
        }).unwrap();
        
        // Best entry should be 500
        let state = pool.get_state();
        assert_eq!(state.books[0].best_entry().unwrap().portion, 500);
    }
    
    #[test]
    fn test_market_entry_no_match_doesnt_rest() {
        let mut pool = EntryPool::new(1000, 2); // 4 lineups
        
        // Submit valid entries to only 2 lineups, with quantities that don't allow matching
        let submit_zero_quantity = pool.submit_entry(0, EntryParameters {
            entry_id: 0,
            entry_type: EntryType::Limit,
            portion: 250,
            quantity: 0, // No quantity available, submission should fail
            self_match_id: None,
        });
        assert!(submit_zero_quantity.is_err());

        pool.submit_entry(1, EntryParameters {
            entry_id: 1,
            entry_type: EntryType::Limit,
            portion: 250,
            quantity: 1,
            self_match_id: None,
        }).unwrap();
        pool.submit_entry(2, EntryParameters {
            entry_id: 2,
            entry_type: EntryType::Limit,
            portion: 250,
            quantity: 1,
            self_match_id: None,
        }).unwrap();

        // Market entry should calculate portion = 500
        let submit_no_match = pool.submit_entry(3, EntryParameters {
            entry_id: 3,
            entry_type: EntryType::Market,
            portion: 0,
            quantity: 1,
            self_match_id: None,
        });
        assert!(submit_no_match.is_err());
        
        // Market entry should not be resting on the book, only successful submits should be resting
        let state = pool.get_state();
        assert!(state.books[0].entries.is_empty());
        assert_eq!(state.books[1].entries.len(), 1);
        assert_eq!(state.books[2].entries.len(), 1);
        assert!(state.books[3].entries.is_empty());
    }
    
    #[test]
    fn test_market_entry_partial_fill_cleared() {
        let mut pool = EntryPool::new(1000, 2); // 4 lineups
        
        // Submit passive entries with quantity=1
        pool.submit_entry(0, EntryParameters {
            entry_id: 0,
            entry_type: EntryType::Limit,
            portion: 250,
            quantity: 1,
            self_match_id: None,
        }).unwrap();
        pool.submit_entry(1, EntryParameters {
            entry_id: 1,
            entry_type: EntryType::Limit,
            portion: 250,
            quantity: 1,
            self_match_id: None,
        }).unwrap();
        pool.submit_entry(2, EntryParameters {
            entry_id: 2,
            entry_type: EntryType::Limit,
            portion: 250,
            quantity: 1,
            self_match_id: None,
        }).unwrap();

        // Market entry with quantity=3 should only match once (limited by passive entries)
        let submit = pool.submit_entry(3, EntryParameters {
            entry_id: 3,
            entry_type: EntryType::Market,
            portion: 0,
            quantity: 3,
            self_match_id: None,
        }).unwrap();
        
        // Should have 1 match
        assert_eq!(submit.match_infos.len(), 1);
        assert_eq!(submit.match_infos[0].matched_quantity, 1);

        // Market entry had quantity=3 but only matched once, so it still had remaining quantity
        // and should be in cancelled_market_entry_ids
        assert_eq!(submit.cancelled_market_entry_ids.len(), 1);
        assert_eq!(submit.cancelled_market_entry_ids[0], 3);

        // Market entry should not be resting on the book (even though it had quantity=3, only 1 matched)
        let state = pool.get_state();
        assert!(!state.books[3].entries.iter().any(|e| e.id == 3));

        // All other entries should also be consumed
        for book in &state.books {
            assert_eq!(book.entries.len(), 0);
        }
    }
    
    #[test]
    fn test_passive_entry_reduction() {
        let mut pool = EntryPool::new(1000, 2); // 4 lineups
        
        // Create the scenario: 3 passive entries at portion=999 each
        pool.submit_entry(1, EntryParameters {
            entry_id: 1,
            entry_type: EntryType::Limit,
            portion: 999,
            quantity: 1,
            self_match_id: None,
        }).unwrap(); // sequence: 0
        pool.submit_entry(2, EntryParameters {
            entry_id: 2,
            entry_type: EntryType::Limit,
            portion: 999,
            quantity: 1,
            self_match_id: None,
        }).unwrap(); // sequence: 1
        pool.submit_entry(3, EntryParameters {
            entry_id: 3,
            entry_type: EntryType::Limit,
            portion: 999,
            quantity: 1,
            self_match_id: None,
        }).unwrap(); // sequence: 2

        // Submit market entry to lineup 0 (will be aggressor with sequence: 3)
        let submit = pool.submit_entry(0, EntryParameters {
            entry_id: 0,
            entry_type: EntryType::Market,
            portion: 0, // ignored
            quantity: 1,
            self_match_id: None,
        }).unwrap();
        
        // Should have 1 match
        assert_eq!(submit.match_infos.len(), 1);
        let match_info =&submit.match_infos[0];
        
        // Aggressor (lineup 0, sequence 3) should have portion=1
        let aggressor_fill = match_info.filled_entries.iter()
            .find(|f| f.entry.lineup_index == 0)
            .unwrap();
        assert_eq!(aggressor_fill.matched_portion, 1);
        
        // Passive entries should be reduced from oldest to newest
        // sequence 0 (lineup 1) should have portion=1
        let passive0 = match_info.filled_entries.iter()
            .find(|f| f.entry.lineup_index == 1)
            .unwrap();
        assert_eq!(passive0.original_portion, 999);
        assert_eq!(passive0.matched_portion, 1);
        
        // sequence 1 (lineup 2) should have portion=1
        let passive1 = match_info.filled_entries.iter()
            .find(|f| f.entry.lineup_index == 2)
            .unwrap();
        assert_eq!(passive1.original_portion, 999);
        assert_eq!(passive1.matched_portion, 1);
        
        // sequence 2 (lineup 3) should have portion=997 (total must equal 1000)
        let passive2 = match_info.filled_entries.iter()
            .find(|f| f.entry.lineup_index == 3)
            .unwrap();
        assert_eq!(passive2.original_portion, 999);
        assert_eq!(passive2.matched_portion, 997);
        
        // Verify total portions = 1000
        let total: u64 = match_info.filled_entries.iter()
            .map(|f| f.matched_portion)
            .sum();
        assert_eq!(total, 1000);
    }
    
    #[test]
    fn test_passive_entry_reduction_with_limit_aggressor() {
        let mut pool = EntryPool::new(1000, 2); // 4 lineups
        
        // Create 3 passive entries at portion=400 each (total=1200)
        pool.submit_entry(1, EntryParameters {
            entry_id: 1,
            entry_type: EntryType::Limit,
            portion: 400,
            quantity: 1,
            self_match_id: None,
        }).unwrap(); // sequence: 0
        pool.submit_entry(2, EntryParameters {
            entry_id: 2,
            entry_type: EntryType::Limit,
            portion: 400,
            quantity: 1,
            self_match_id: None,
        }).unwrap(); // sequence: 1
        pool.submit_entry(3, EntryParameters {
            entry_id: 3,
            entry_type: EntryType::Limit,
            portion: 400,
            quantity: 1,
            self_match_id: None,
        }).unwrap(); // sequence: 2

        // Submit limit entry with portion=300 to lineup 0 (will be aggressor)
        let submit = pool.submit_entry(0, EntryParameters {
            entry_id: 0,
            entry_type: EntryType::Limit,
            portion: 300,
            quantity: 1,
            self_match_id: None,
        }).unwrap();
        
        // Should have 1 match
        assert_eq!(submit.match_infos.len(), 1);
        let match_info =&submit.match_infos[0];
        
        // Total without adjustment would be 1500, need to reduce by 500
        // First reduce aggressor: 300 -> 1 (reduction of 299)
        // Remaining excess: 500 - 299 = 201
        // Then reduce passives from oldest to newest:
        //   seq 0: 400 -> 1 (reduction of 399, but we only need 201)
        //   So seq 0: 400 -> 199
        
        let aggressor_fill = match_info.filled_entries.iter()
            .find(|f| f.entry.lineup_index == 0)
            .unwrap();
        assert_eq!(aggressor_fill.matched_portion, 1);
        
        let passive0 = match_info.filled_entries.iter()
            .find(|f| f.entry.lineup_index == 1)
            .unwrap();
        assert_eq!(passive0.matched_portion, 199);
        
        let passive1 = match_info.filled_entries.iter()
            .find(|f| f.entry.lineup_index == 2)
            .unwrap();
        assert_eq!(passive1.matched_portion, 400);
        
        let passive2 = match_info.filled_entries.iter()
            .find(|f| f.entry.lineup_index == 3)
            .unwrap();
        assert_eq!(passive2.matched_portion, 400);
        
        // Verify total = 1000
        let total: u64 = match_info.filled_entries.iter()
            .map(|f| f.matched_portion)
            .sum();
        assert_eq!(total, 1000);
    }
    
    #[test]
    fn test_cancel_entry_success() {
        let mut pool = EntryPool::new(1000, 2); // 4 lineups

        // Submit an entry
        pool.submit_entry(0, EntryParameters {
            entry_id: 100,
            entry_type: EntryType::Limit,
            portion: 250,
            quantity: 1,
            self_match_id: None,
        }).unwrap();

        // Verify entry is in the book
        let state = pool.get_state();
        assert_eq!(state.books[0].entries.len(), 1);
        assert_eq!(state.books[0].entries[0].id, 100);

        // Cancel the entry
        let result = pool.cancel_entry(100);
        assert!(result.is_ok());

        // Verify entry is removed from the book
        let state = pool.get_state();
        assert_eq!(state.books[0].entries.len(), 0);
    }

    #[test]
    fn test_cancel_entry_not_found() {
        let mut pool = EntryPool::new(1000, 2); // 4 lineups

        // Try to cancel an entry that doesn't exist
        let result = pool.cancel_entry(999);
        assert!(result.is_err());
        assert_eq!(result.unwrap_err(), "Entry with ID 999 not found");
    }

    #[test]
    fn test_cancelled_entry_does_not_fill() {
        let mut pool = EntryPool::new(1000, 2); // 4 lineups

        // Submit entries to all 4 lineups
        pool.submit_entry(0, EntryParameters {
            entry_id: 100,
            entry_type: EntryType::Limit,
            portion: 250,
            quantity: 1,
            self_match_id: None,
        }).unwrap();
        pool.submit_entry(1, EntryParameters {
            entry_id: 101,
            entry_type: EntryType::Limit,
            portion: 250,
            quantity: 1,
            self_match_id: None,
        }).unwrap();
        pool.submit_entry(2, EntryParameters {
            entry_id: 102,
            entry_type: EntryType::Limit,
            portion: 250,
            quantity: 1,
            self_match_id: None,
        }).unwrap();
        pool.submit_entry(3, EntryParameters {
            entry_id: 103,
            entry_type: EntryType::Limit,
            portion: 250,
            quantity: 1,
            self_match_id: None,
        }).unwrap();

        // At this point, we should have had a fill, so all books are empty
        let state = pool.get_state();
        for book in &state.books {
            assert_eq!(book.entries.len(), 0);
        }

        // Submit entries again, but this time cancel one before completing
        pool.submit_entry(0, EntryParameters {
            entry_id: 200,
            entry_type: EntryType::Limit,
            portion: 250,
            quantity: 1,
            self_match_id: None,
        }).unwrap();
        pool.submit_entry(1, EntryParameters {
            entry_id: 201,
            entry_type: EntryType::Limit,
            portion: 250,
            quantity: 1,
            self_match_id: None,
        }).unwrap();
        pool.submit_entry(2, EntryParameters {
            entry_id: 202,
            entry_type: EntryType::Limit,
            portion: 250,
            quantity: 1,
            self_match_id: None,
        }).unwrap();

        // Cancel entry 202
        pool.cancel_entry(202).unwrap();

        // Now submit the final entry - should NOT trigger a fill because entry 202 is cancelled
        let submit = pool.submit_entry(3, EntryParameters {
            entry_id: 203,
            entry_type: EntryType::Limit,
            portion: 250,
            quantity: 1,
            self_match_id: None,
        }).unwrap();

        // Should have NO matches because lineup 2 has no entries
        assert_eq!(submit.match_infos.len(), 0);

        // Verify entries are still on the book (except the cancelled one)
        let state = pool.get_state();
        assert!(state.books[0].entries.iter().any(|e| e.id == 200));
        assert!(state.books[1].entries.iter().any(|e| e.id == 201));
        assert!(!state.books[2].entries.iter().any(|e| e.id == 202)); // Cancelled
        assert!(state.books[3].entries.iter().any(|e| e.id == 203));
    }

    #[test]
    fn test_cancel_entry_with_multiple_in_same_book() {
        let mut pool = EntryPool::new(1000, 2); // 4 lineups

        // Submit multiple entries to the same lineup
        pool.submit_entry(0, EntryParameters {
            entry_id: 100,
            entry_type: EntryType::Limit,
            portion: 300,
            quantity: 1,
            self_match_id: None,
        }).unwrap();
        pool.submit_entry(0, EntryParameters {
            entry_id: 101,
            entry_type: EntryType::Limit,
            portion: 250,
            quantity: 1,
            self_match_id: None,
        }).unwrap();
        pool.submit_entry(0, EntryParameters {
            entry_id: 102,
            entry_type: EntryType::Limit,
            portion: 200,
            quantity: 1,
            self_match_id: None,
        }).unwrap();

        // Verify all 3 entries are in the book
        let state = pool.get_state();
        assert_eq!(state.books[0].entries.len(), 3);

        // Cancel the middle entry (by portion, not by position)
        pool.cancel_entry(101).unwrap();

        // Verify only 2 entries remain
        let state = pool.get_state();
        assert_eq!(state.books[0].entries.len(), 2);
        assert!(state.books[0].entries.iter().any(|e| e.id == 100));
        assert!(!state.books[0].entries.iter().any(|e| e.id == 101)); // Cancelled
        assert!(state.books[0].entries.iter().any(|e| e.id == 102));
    }

    #[test]
    fn test_is_complete_single_fill() {
        let mut pool = EntryPool::new(1000, 2); // 4 lineups

        // Submit entries with quantity=1 each - will all be completed after one fill
        pool.submit_entry(0, EntryParameters {
            entry_id: 100,
            entry_type: EntryType::Limit,
            portion: 250,
            quantity: 1,
            self_match_id: None,
        }).unwrap();
        pool.submit_entry(1, EntryParameters {
            entry_id: 101,
            entry_type: EntryType::Limit,
            portion: 250,
            quantity: 1,
            self_match_id: None,
        }).unwrap();
        pool.submit_entry(2, EntryParameters {
            entry_id: 102,
            entry_type: EntryType::Limit,
            portion: 250,
            quantity: 1,
            self_match_id: None,
        }).unwrap();
        let submit = pool.submit_entry(3, EntryParameters {
            entry_id: 103,
            entry_type: EntryType::Limit,
            portion: 250,
            quantity: 1,
            self_match_id: None,
        }).unwrap();

        // Should have 1 match
        assert_eq!(submit.match_infos.len(), 1);

        // All entries should be completed, so is_complete should be true for all
        let match_info =&submit.match_infos[0];
        for filled_entry in &match_info.filled_entries {
            assert!(filled_entry.is_complete,
                "Entry {} should have is_complete=true", filled_entry.entry.id);
        }
    }

    #[test]
    fn test_is_complete_partial_fill() {
        let mut pool = EntryPool::new(1000, 2); // 4 lineups

        // Submit entries with quantity=2 each, except one with quantity=1
        pool.submit_entry(0, EntryParameters {
            entry_id: 100,
            entry_type: EntryType::Limit,
            portion: 250,
            quantity: 2,
            self_match_id: None,
        }).unwrap();
        pool.submit_entry(1, EntryParameters {
            entry_id: 101,
            entry_type: EntryType::Limit,
            portion: 250,
            quantity: 2,
            self_match_id: None,
        }).unwrap();
        pool.submit_entry(2, EntryParameters {
            entry_id: 102,
            entry_type: EntryType::Limit,
            portion: 250,
            quantity: 2,
            self_match_id: None,
        }).unwrap();
        let submit = pool.submit_entry(3, EntryParameters {
            entry_id: 103,
            entry_type: EntryType::Limit,
            portion: 250,
            quantity: 1, // This one will be completed
            self_match_id: None,
        }).unwrap();

        // Should have 1 match
        assert_eq!(submit.match_infos.len(), 1);

        // Only entry 103 should have is_complete=true
        let match_info =&submit.match_infos[0];
        for filled_entry in &match_info.filled_entries {
            if filled_entry.entry.id == 103 {
                assert!(filled_entry.is_complete,
                    "Entry 103 should have is_complete=true");
            } else {
                assert!(!filled_entry.is_complete,
                    "Entry {} should have is_complete=false", filled_entry.entry.id);
            }
        }
    }

    #[test]
    fn test_is_complete_multiple_fills_aggressor() {
        let mut pool = EntryPool::new(1000, 2); // 4 lineups

        // Submit passive entries with quantity=2 each
        pool.submit_entry(0, EntryParameters {
            entry_id: 100,
            entry_type: EntryType::Limit,
            portion: 250,
            quantity: 2,
            self_match_id: None,
        }).unwrap();
        pool.submit_entry(1, EntryParameters {
            entry_id: 101,
            entry_type: EntryType::Limit,
            portion: 250,
            quantity: 2,
            self_match_id: None,
        }).unwrap();
        // Add TWO entries to lineup 2 to enable 2 matches
        pool.submit_entry(2, EntryParameters {
            entry_id: 102,
            entry_type: EntryType::Limit,
            portion: 250,
            quantity: 1,
            self_match_id: None,
        }).unwrap();
        pool.submit_entry(2, EntryParameters {
            entry_id: 103,
            entry_type: EntryType::Limit,
            portion: 250,
            quantity: 1,
            self_match_id: None,
        }).unwrap();

        // Aggressor with quantity=2 will trigger 2 matches
        let submit = pool.submit_entry(3, EntryParameters {
            entry_id: 104,
            entry_type: EntryType::Limit,
            portion: 250,
            quantity: 2,
            self_match_id: None,
        }).unwrap();

        // Should have 2 matches
        assert_eq!(submit.match_infos.len(), 2);

        // Match 0: entries 100, 101, 102, 104
        // Only entry 102 is completed after this fill
        let match_info_0 = &submit.match_infos[0];
        for filled_entry in &match_info_0.filled_entries {
            match filled_entry.entry.id {
                102 => {
                    assert!(filled_entry.is_complete,
                        "Entry 102 should have is_complete=true (completed in first fill)");
                }
                100 | 101 | 104 => {
                    assert!(!filled_entry.is_complete,
                        "Entry {} should have is_complete=false in first match",
                        filled_entry.entry.id);
                }
                _ => panic!("Unexpected entry in match 0"),
            }
        }

        // Match 1: entries 100, 101, 103, 104
        // All entries are completed after this fill
        let match_info_1 = &submit.match_infos[1];
        for filled_entry in &match_info_1.filled_entries {
            assert!(filled_entry.is_complete,
                "Entry {} should have is_complete=true in second (final) match",
                filled_entry.entry.id);
        }
    }

    #[test]
    fn test_is_complete_resting_order_multiple_fills() {
        // This test specifically addresses the requirement:
        // "if a resting (non-aggressor) order gets filled multiple times as the result of one OrderNew,
        // but only the final match should have "isComplete" as true for that order"
        let mut pool = EntryPool::new(1000, 2); // 4 lineups

        // Submit TWO entries to lineup 0 with quantity=1 each (to create 2 matches)
        pool.submit_entry(0, EntryParameters {
            entry_id: 100,
            entry_type: EntryType::Limit,
            portion: 250,
            quantity: 1,
            self_match_id: None,
        }).unwrap();
        pool.submit_entry(0, EntryParameters {
            entry_id: 101,
            entry_type: EntryType::Limit,
            portion: 250,
            quantity: 1,
            self_match_id: None,
        }).unwrap();

        // Submit resting entries to lineups 1 and 2 with quantity=2 each
        // These will participate in BOTH matches
        pool.submit_entry(1, EntryParameters {
            entry_id: 102,
            entry_type: EntryType::Limit,
            portion: 250,
            quantity: 2,
            self_match_id: None,
        }).unwrap();
        pool.submit_entry(2, EntryParameters {
            entry_id: 103,
            entry_type: EntryType::Limit,
            portion: 250,
            quantity: 2,
            self_match_id: None,
        }).unwrap();

        // Aggressor with quantity=2 will trigger 2 matches
        let submit = pool.submit_entry(3, EntryParameters {
            entry_id: 104,
            entry_type: EntryType::Limit,
            portion: 250,
            quantity: 2,
            self_match_id: None,
        }).unwrap();

        // Should have 2 matches
        assert_eq!(submit.match_infos.len(), 2);

        // Match 0: entries 100, 102, 103, 104
        let match_info_0 = &submit.match_infos[0];
        for filled_entry in &match_info_0.filled_entries {
            match filled_entry.entry.id {
                100 => {
                    // Entry 100 is completed after first fill
                    assert!(filled_entry.is_complete,
                        "Entry 100 should have is_complete=true (completed in first fill)");
                }
                102 | 103 => {
                    // Resting orders 102 and 103 are NOT completed after first fill
                    assert!(!filled_entry.is_complete,
                        "Resting entry {} should have is_complete=false in first match",
                        filled_entry.entry.id);
                }
                104 => {
                    // Aggressor 104 is NOT completed after first fill
                    assert!(!filled_entry.is_complete,
                        "Aggressor entry 104 should have is_complete=false in first match");
                }
                _ => panic!("Unexpected entry in match 0"),
            }
        }

        // Match 1: entries 101, 102, 103, 104
        let match_info_1 = &submit.match_infos[1];
        for filled_entry in &match_info_1.filled_entries {
            match filled_entry.entry.id {
                101 | 102 | 103 | 104 => {
                    // All entries are completed after second fill
                    assert!(filled_entry.is_complete,
                        "Entry {} should have is_complete=true in second (final) match",
                        filled_entry.entry.id);
                }
                _ => panic!("Unexpected entry in match 1"),
            }
        }
    }

    #[test]
    fn test_self_match_id_cancels_other_lineups() {
        let mut pool = EntryPool::new(1000, 2); // 4 lineups

        // Submit entries to lineups 1, 2, and 3 with different self_match_ids initially
        pool.submit_entry(1, EntryParameters {
            entry_id: 101,
            entry_type: EntryType::Limit,
            portion: 250,
            quantity: 1,
            self_match_id: Some(42),
        }).unwrap();
        pool.submit_entry(2, EntryParameters {
            entry_id: 102,
            entry_type: EntryType::Limit,
            portion: 250,
            quantity: 1,
            self_match_id: Some(42),
        }).unwrap();
        // Entry 102 should have cancelled entry 101
        let state = pool.get_state();
        assert_eq!(state.books[1].entries.len(), 0); // 101 was cancelled by 102
        assert_eq!(state.books[2].entries.len(), 1); // 102 remains

        pool.submit_entry(3, EntryParameters {
            entry_id: 103,
            entry_type: EntryType::Limit,
            portion: 250,
            quantity: 1,
            self_match_id: Some(42),
        }).unwrap();
        // Entry 103 should have cancelled entry 102
        let state = pool.get_state();
        assert_eq!(state.books[2].entries.len(), 0); // 102 was cancelled by 103
        assert_eq!(state.books[3].entries.len(), 1); // 103 remains

        // Submit entry to lineup 0 with same self_match_id
        let submit = pool.submit_entry(0, EntryParameters {
            entry_id: 100,
            entry_type: EntryType::Limit,
            portion: 250,
            quantity: 1,
            self_match_id: Some(42),
        }).unwrap();

        // Should have cancelled only entry 103 (the only one remaining)
        assert_eq!(submit.cancelled_entry_ids.len(), 1);
        assert!(submit.cancelled_entry_ids.contains(&103));

        // Verify entries are removed from books
        let state = pool.get_state();
        assert_eq!(state.books[0].entries.len(), 1); // Only new entry 100
        assert_eq!(state.books[1].entries.len(), 0); // 101 cancelled earlier
        assert_eq!(state.books[2].entries.len(), 0); // 102 cancelled earlier
        assert_eq!(state.books[3].entries.len(), 0); // 103 cancelled
    }

    #[test]
    fn test_self_match_id_same_lineup_allowed() {
        let mut pool = EntryPool::new(1000, 2); // 4 lineups

        // Submit multiple entries to the same lineup (0) with same self_match_id
        pool.submit_entry(0, EntryParameters {
            entry_id: 100,
            entry_type: EntryType::Limit,
            portion: 300,
            quantity: 1,
            self_match_id: Some(42),
        }).unwrap();
        pool.submit_entry(0, EntryParameters {
            entry_id: 101,
            entry_type: EntryType::Limit,
            portion: 250,
            quantity: 1,
            self_match_id: Some(42),
        }).unwrap();
        let submit = pool.submit_entry(0, EntryParameters {
            entry_id: 102,
            entry_type: EntryType::Limit,
            portion: 200,
            quantity: 1,
            self_match_id: Some(42),
        }).unwrap();

        // No cancellations should occur (same lineup)
        assert_eq!(submit.cancelled_entry_ids.len(), 0);

        // All 3 entries should coexist in lineup 0
        let state = pool.get_state();
        assert_eq!(state.books[0].entries.len(), 3);
        assert!(state.books[0].entries.iter().any(|e| e.id == 100));
        assert!(state.books[0].entries.iter().any(|e| e.id == 101));
        assert!(state.books[0].entries.iter().any(|e| e.id == 102));
    }

    #[test]
    fn test_self_match_id_none_no_cancellation() {
        let mut pool = EntryPool::new(1000, 2); // 4 lineups

        // Submit entries to different lineups with self_match_id=None
        pool.submit_entry(0, EntryParameters {
            entry_id: 100,
            entry_type: EntryType::Limit,
            portion: 250,
            quantity: 1,
            self_match_id: None,
        }).unwrap();
        pool.submit_entry(1, EntryParameters {
            entry_id: 101,
            entry_type: EntryType::Limit,
            portion: 250,
            quantity: 1,
            self_match_id: None,
        }).unwrap();
        let submit = pool.submit_entry(2, EntryParameters {
            entry_id: 102,
            entry_type: EntryType::Limit,
            portion: 250,
            quantity: 1,
            self_match_id: None,
        }).unwrap();

        // No cancellations should occur
        assert_eq!(submit.cancelled_entry_ids.len(), 0);

        // All entries should remain
        let state = pool.get_state();
        assert_eq!(state.books[0].entries.len(), 1);
        assert_eq!(state.books[1].entries.len(), 1);
        assert_eq!(state.books[2].entries.len(), 1);
    }

    #[test]
    fn test_self_match_id_different_ids_no_cancellation() {
        let mut pool = EntryPool::new(1000, 2); // 4 lineups

        // Submit entries to different lineups with different self_match_ids
        pool.submit_entry(0, EntryParameters {
            entry_id: 100,
            entry_type: EntryType::Limit,
            portion: 250,
            quantity: 1,
            self_match_id: Some(1),
        }).unwrap();
        pool.submit_entry(1, EntryParameters {
            entry_id: 101,
            entry_type: EntryType::Limit,
            portion: 250,
            quantity: 1,
            self_match_id: Some(2),
        }).unwrap();
        let submit = pool.submit_entry(2, EntryParameters {
            entry_id: 102,
            entry_type: EntryType::Limit,
            portion: 250,
            quantity: 1,
            self_match_id: Some(3),
        }).unwrap();

        // No cancellations should occur (different IDs)
        assert_eq!(submit.cancelled_entry_ids.len(), 0);

        // All entries should remain
        let state = pool.get_state();
        assert_eq!(state.books[0].entries.len(), 1);
        assert_eq!(state.books[1].entries.len(), 1);
        assert_eq!(state.books[2].entries.len(), 1);
    }

    #[test]
    fn test_self_match_id_cancellation_then_match() {
        let mut pool = EntryPool::new(1000, 2); // 4 lineups

        // Submit entries to lineups 0, 1, 2 with different self_match_ids to avoid cancelling each other
        pool.submit_entry(0, EntryParameters {
            entry_id: 100,
            entry_type: EntryType::Limit,
            portion: 250,
            quantity: 1,
            self_match_id: Some(1),
        }).unwrap();
        pool.submit_entry(1, EntryParameters {
            entry_id: 101,
            entry_type: EntryType::Limit,
            portion: 250,
            quantity: 1,
            self_match_id: Some(2),
        }).unwrap();
        pool.submit_entry(2, EntryParameters {
            entry_id: 102,
            entry_type: EntryType::Limit,
            portion: 250,
            quantity: 1,
            self_match_id: Some(3),
        }).unwrap();

        // Verify all entries are resting
        let state = pool.get_state();
        assert_eq!(state.books[0].entries.len(), 1);
        assert_eq!(state.books[1].entries.len(), 1);
        assert_eq!(state.books[2].entries.len(), 1);

        // Now submit to lineup 3 with self_match_id=1, should cancel entry 100
        let submit3 = pool.submit_entry(3, EntryParameters {
            entry_id: 103,
            entry_type: EntryType::Limit,
            portion: 250,
            quantity: 1,
            self_match_id: Some(1),
        }).unwrap();

        // Should have cancelled entry 100
        assert_eq!(submit3.cancelled_entry_ids.len(), 1);
        assert!(submit3.cancelled_entry_ids.contains(&100));

        // Entry 100 should be cancelled, 101, 102, 103 are resting
        let state = pool.get_state();
        assert_eq!(state.books[0].entries.len(), 0);
        assert_eq!(state.books[1].entries.len(), 1);
        assert_eq!(state.books[2].entries.len(), 1);
        assert_eq!(state.books[3].entries.len(), 1);

        // Now submit fresh entry to lineup 0 with different self_match_id to enable matching
        let submit = pool.submit_entry(0, EntryParameters {
            entry_id: 200,
            entry_type: EntryType::Limit,
            portion: 250,
            quantity: 1,
            self_match_id: Some(99),
        }).unwrap();

        // Should trigger a match with entries 200, 101, 102, 103
        assert_eq!(submit.match_infos.len(), 1);
    }

    #[test]
    fn test_self_match_id_partial_cancellation() {
        let mut pool = EntryPool::new(1000, 2); // 4 lineups

        // Submit entries with mixed self_match_ids
        pool.submit_entry(1, EntryParameters {
            entry_id: 101,
            entry_type: EntryType::Limit,
            portion: 250,
            quantity: 1,
            self_match_id: Some(42),
        }).unwrap();
        pool.submit_entry(2, EntryParameters {
            entry_id: 102,
            entry_type: EntryType::Limit,
            portion: 250,
            quantity: 1,
            self_match_id: Some(99), // Different ID
        }).unwrap();
        pool.submit_entry(3, EntryParameters {
            entry_id: 103,
            entry_type: EntryType::Limit,
            portion: 250,
            quantity: 1,
            self_match_id: None, // No ID
        }).unwrap();

        // Submit entry to lineup 0 with self_match_id=Some(42)
        let submit = pool.submit_entry(0, EntryParameters {
            entry_id: 100,
            entry_type: EntryType::Limit,
            portion: 250,
            quantity: 1,
            self_match_id: Some(42),
        }).unwrap();

        // Should only cancel entry 101
        assert_eq!(submit.cancelled_entry_ids.len(), 1);
        assert!(submit.cancelled_entry_ids.contains(&101));

        // Entries 102 and 103 should remain
        let state = pool.get_state();
        assert_eq!(state.books[1].entries.len(), 0); // 101 cancelled
        assert_eq!(state.books[2].entries.len(), 1); // 102 remains
        assert_eq!(state.books[3].entries.len(), 1); // 103 remains
    }

    #[test]
    fn test_self_match_id_market_entry_cancellation() {
        let mut pool = EntryPool::new(1000, 2); // 4 lineups

        // Submit limit entries to lineups 0, 1, 2 with different self_match_ids to avoid cancelling each other
        pool.submit_entry(0, EntryParameters {
            entry_id: 100,
            entry_type: EntryType::Limit,
            portion: 200,
            quantity: 1,
            self_match_id: Some(1),
        }).unwrap();
        pool.submit_entry(1, EntryParameters {
            entry_id: 101,
            entry_type: EntryType::Limit,
            portion: 250,
            quantity: 1,
            self_match_id: Some(2),
        }).unwrap();
        pool.submit_entry(2, EntryParameters {
            entry_id: 102,
            entry_type: EntryType::Limit,
            portion: 250,
            quantity: 1,
            self_match_id: Some(3),
        }).unwrap();

        // Verify all entries are resting
        let state = pool.get_state();
        assert_eq!(state.books[0].entries.len(), 1);
        assert_eq!(state.books[1].entries.len(), 1);
        assert_eq!(state.books[2].entries.len(), 1);

        // Submit market entry to lineup 3 with self_match_id=1
        // Market entry will calculate portion based on existing entries (including 100),
        // then cancel entry 100, then try to match but fail silently (no matches)
        let submit = pool.submit_entry(3, EntryParameters {
            entry_id: 103,
            entry_type: EntryType::Market,
            portion: 0,
            quantity: 1,
            self_match_id: Some(1),
        }).unwrap();

        // Submission succeeds but with no matches (market entry doesn't match)
        assert_eq!(submit.match_infos.len(), 0);
        // Entry 100 should be in cancelled_entry_ids
        assert_eq!(submit.cancelled_entry_ids.len(), 1);
        assert!(submit.cancelled_entry_ids.contains(&100));

        // Entry 100 should be cancelled, market entry not resting (doesn't match)
        let state = pool.get_state();
        assert_eq!(state.books[0].entries.len(), 0);
        assert_eq!(state.books[1].entries.len(), 1); // 101 remains
        assert_eq!(state.books[2].entries.len(), 1); // 102 remains
        assert_eq!(state.books[3].entries.len(), 0); // Market entry not rested
    }

    #[test]
    fn test_self_match_id_multiple_entries_same_id_cancelled() {
        let mut pool = EntryPool::new(1000, 2); // 4 lineups

        // Submit multiple entries to lineup 1 with same self_match_id
        // They should NOT cancel each other (same lineup)
        pool.submit_entry(1, EntryParameters {
            entry_id: 101,
            entry_type: EntryType::Limit,
            portion: 300,
            quantity: 1,
            self_match_id: Some(42),
        }).unwrap();
        pool.submit_entry(1, EntryParameters {
            entry_id: 102,
            entry_type: EntryType::Limit,
            portion: 250,
            quantity: 1,
            self_match_id: Some(42),
        }).unwrap();

        // Both entries should be in lineup 1
        let state = pool.get_state();
        assert_eq!(state.books[1].entries.len(), 2);

        // Submit entry to lineup 2 with same self_match_id
        // Should cancel both 101 and 102 (different lineup)
        let submit2 = pool.submit_entry(2, EntryParameters {
            entry_id: 201,
            entry_type: EntryType::Limit,
            portion: 200,
            quantity: 1,
            self_match_id: Some(42),
        }).unwrap();

        // Should have cancelled both entries in lineup 1
        assert_eq!(submit2.cancelled_entry_ids.len(), 2);
        assert!(submit2.cancelled_entry_ids.contains(&101));
        assert!(submit2.cancelled_entry_ids.contains(&102));

        // Only entry 201 should remain
        let state = pool.get_state();
        assert_eq!(state.books[1].entries.len(), 0);
        assert_eq!(state.books[2].entries.len(), 1);

        // Submit entry to lineup 0 with same self_match_id
        let submit = pool.submit_entry(0, EntryParameters {
            entry_id: 100,
            entry_type: EntryType::Limit,
            portion: 250,
            quantity: 1,
            self_match_id: Some(42),
        }).unwrap();

        // Should cancel only entry 201
        assert_eq!(submit.cancelled_entry_ids.len(), 1);
        assert!(submit.cancelled_entry_ids.contains(&201));

        // Verify only 100 remains
        let state = pool.get_state();
        assert_eq!(state.books[0].entries.len(), 1); // Only 100
        assert_eq!(state.books[1].entries.len(), 0);
        assert_eq!(state.books[2].entries.len(), 0);
    }
}