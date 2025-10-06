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
// - When the best entry from each lineup can form a valid pool, a "fill event" occurs
// - In one fill event, there are 2^N fills (one from each entry in each lineup)
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
    /// Total quantity available for matching (must be multiple of portion)
    pub quantity: u64,
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
    /// Total quantity available for matching (must be multiple of portion)
    pub quantity: u64,
    /// Timestamp/sequence for FIFO ordering among same-portion entries
    pub sequence: u64,
}

impl Entry {
    /// Returns true if this entry has enough quantity remaining to stay in the book
    pub fn has_remaining_quantity(&self) -> bool {
        self.quantity >= self.portion
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
    pub fn remove_market_entries(&mut self) {
        self.entries.retain(|entry| entry.entry_type != EntryType::Market);
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

/// Represents a single fill in a fill event, including original and matched portions
#[derive(Debug, Clone)]
pub struct FilledEntry {
    /// The entry that was matched
    pub entry: Entry,
    /// Original portion before any aggressor adjustment
    pub original_portion: u64,
    /// matched portion after aggressor adjustment (may differ from original for aggressor)
    pub matched_portion: u64,
    /// Quantity that was matched in this fill
    pub matched_quantity: u64,
}

/// Represents a successful fill event across all lineups
#[derive(Debug, Clone)]
pub struct FillEvent {
    /// One filled entry from each lineup (2^N entries total)
    pub filled_entries: Vec<FilledEntry>,
    /// Index of the lineup that was the aggressor
    pub aggressor_lineup_index: usize,
}

impl fmt::Display for FillEvent {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        writeln!(f, "FillEvent (Aggressor: Lineup {}):", self.aggressor_lineup_index)?;
        for fill in &self.filled_entries {
            let aggressor_marker = if fill.entry.lineup_index == self.aggressor_lineup_index {
                " [AGGRESSOR]"
            } else {
                ""
            };
            writeln!(
                f,
                "  Lineup {}: ID:{} portion:{}→{} matched_qty:{}{}",
                fill.entry.lineup_index,
                fill.entry.id,
                fill.original_portion,
                fill.matched_portion,
                fill.matched_quantity,
                aggressor_marker
            )?;
        }
        Ok(())
    }
}

/// Information returned when an entry is successfully submitted
#[derive(Debug, Clone)]
pub struct SubmitInfo {
    /// Any fill events that occurred as a result of this submission
    pub fill_events: Vec<FillEvent>,
    /// All entries that are no longer resting on the book as a result of this submitted entry (includes the sunbmitted entry if applicable)
    pub completed_entry_ids: Vec<u64>,
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
    /// * `Ok(SubmitInfo)` - Contains the entry ID and any fill events that occurred
    /// * `Err(String)` - Error message if submission failed
    ///
    /// For Market entries: attempts immediate matching. If no valid fill exists, 
    /// the market entry is rejected (not rested on book).
    ///
    /// For Limit entries: added to book, then checked for potential fills.
    /// May trigger multiple fill events if quantity is large enough.
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
        
        // Handle market vs limit entries differently
        match params.entry_type {
            EntryType::Market => self.submit_market_entry(params.entry_id, lineup_index, params.quantity),
            EntryType::Limit => self.submit_limit_entry(params.entry_id, lineup_index, params.portion, params.quantity),
        }
    }
    
    /// Submits a market entry - attempts immediate fill or rejects
    fn submit_market_entry(&mut self, entry_id: u64, lineup_index: usize, quantity: u64) -> SubmitResult {
        // Calculate what portion is needed to complete the pool
        let other_lineups: Vec<usize> = (0..self.state.num_lineups())
            .filter(|&i| i != lineup_index)
            .collect();
        
        // Get best entries from all other lineups
        let mut other_portions = Vec::new();
        for &other_idx in &other_lineups {
            if let Some(entry) = self.state.books[other_idx].best_entry() {
                other_portions.push(entry.portion);
            } else {
                // Can't form a valid fill - missing entries from some lineups
                return Err("Market entry rejected: insufficient passive entries for fill".to_string());
            }
        }
        
        // Calculate needed portion for market entry
        let other_sum: u64 = other_portions.iter().sum();
        
        if other_sum >= self.state.total_units {
            return Err("Market entry rejected: passive entries already exceed or equal total units".to_string());
        }
        
        let market_portion = self.state.total_units - other_sum;
        
        // Validate quantity is multiple of calculated portion
        if quantity % market_portion != 0 {
            return Err(format!(
                "Market entry rejected: quantity {} must be multiple of calculated portion {}",
                quantity, market_portion
            ));
        }
        
        // Create the market entry
        let entry = Entry {
            id: entry_id,
            lineup_index,
            entry_type: EntryType::Market,
            portion: market_portion,
            quantity,
            sequence: self.next_sequence,
        };
        self.next_sequence += 1;
        
        // Add to book
        self.state.books[lineup_index].add_entry(entry);
        
        // Attempt fills (market entries should match immediately)
        let fill_events = self.attempt_fills();

        // Iterate over fillfill_eventsEvents and collect entry IDs that are no longer on the book
        let mut completed_entry_ids = Vec::new();
        for fill_event in &fill_events {
            for filled_entry in &fill_event.filled_entries {
                let lineup_idx = filled_entry.entry.lineup_index;
                let entry_id = filled_entry.entry.id;
                // If the entry is no longer present in the book, add its ID
                let still_on_book = self.state.books[lineup_idx]
                    .entries
                    .iter()
                    .any(|e| e.id == entry_id);
                if !still_on_book {
                    completed_entry_ids.push(entry_id);
                }
            }
        }

        Ok(SubmitInfo {
            fill_events,
            completed_entry_ids
        })
    }
    
    /// Submits a limit entry - adds to book and attempts fills
    fn submit_limit_entry(&mut self, entry_id: u64, lineup_index: usize, portion: u64, quantity: u64) -> SubmitResult {
        // Validate quantity is multiple of portion
        if quantity % portion != 0 {
            return Err(format!(
                "Quantity {} must be a multiple of portion {}",
                quantity, portion
            ));
        }
        
        // Validate portion is positive
        if portion == 0 {
            return Err("Portion must be greater than 0".to_string());
        }
        
        // Create the limit entry
        let entry = Entry {
            id: entry_id,
            lineup_index,
            entry_type: EntryType::Limit,
            portion,
            quantity,
            sequence: self.next_sequence,
        };
        self.next_sequence += 1;
        
        // Add to book
        self.state.books[lineup_index].add_entry(entry);
        
        // Attempt fills - may trigger multiple fill_events
        let fill_events = self.attempt_fills();

        // Iterate over fill_events and collect entry IDs that are no longer on the book
        let mut completed_entry_ids = Vec::new();
        for fill_event in &fill_events {
            for filled_entry in &fill_event.filled_entries {
                let lineup_idx = filled_entry.entry.lineup_index;
                let entry_id = filled_entry.entry.id;
                // If the entry is no longer present in the book, add its ID
                let still_on_book = self.state.books[lineup_idx]
                    .entries
                    .iter()
                    .any(|e| e.id == entry_id);
                if !still_on_book {
                    completed_entry_ids.push(entry_id);
                }
            }
        }
        
        Ok(SubmitInfo {
            fill_events,
            completed_entry_ids
        })
    }
    
    /// Attempts to create fills from the current book state
    /// Continues attempting fills until no valid fill can be made
    fn attempt_fills(&mut self) -> Vec<FillEvent> {
        let mut fill_events = Vec::new();
        
        loop {
            match self.try_create_fill() {
                Some(fill_event) => fill_events.push(fill_event),
                None => break,
            }
        }

        // Remove market entries from all books
        for book in &mut self.state.books {
            book.remove_market_entries();
        }
        
        fill_events
    }
    
    /// Attempts to create a single fill event from best entries across all lineups
    /// Returns Some(FillEvent) if successful, None if no valid fill exists
    fn try_create_fill(&mut self) -> Option<FillEvent> {
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
        
        // Calculate sum of portions
        let total_portion: u64 = best_entries.iter().map(|(_, e)| e.portion).sum();
        
        // Check if we can form a valid fill event
        if total_portion < self.state.total_units {
            // Not enough to fill the pool
            return None;
        }
        
        // Determine which entry is the aggressor (most recent)
        // The aggressor is the entry with the highest sequence number
        let (aggressor_idx, aggressor_lineup_index) = best_entries
            .iter()
            .enumerate()
            .max_by_key(|(_, (_, entry))| entry.sequence)
            .map(|(idx, (lineup_idx, _))| (idx, *lineup_idx))?;
        
        // Calculate portions and quantities for the fill
        let mut filled_entries = Vec::new();
        let mut aggressor_portion_adjustment = 0u64;
        
        if total_portion > self.state.total_units {
            // Need to reduce aggressor's portion
            let excess = total_portion - self.state.total_units;
            aggressor_portion_adjustment = excess;
        }
        
        // Calculate minimum number of times we can match across all entries
        // Each entry can match (quantity / matched_portion) times
        let mut min_times_matchable = u64::MAX;
        
        for (idx, (_lineup_idx, entry)) in best_entries.iter().enumerate() {
            let matched_portion = if idx == aggressor_idx {
                entry.portion.saturating_sub(aggressor_portion_adjustment)
            } else {
                entry.portion
            };
            
            // Can't have zero portion
            if matched_portion == 0 {
                return None;
            }
            
            // Calculate how many times this entry can match with its matched portion
            let times_matchable = entry.quantity / matched_portion;
            min_times_matchable = min_times_matchable.min(times_matchable);
        }
        
        // If no entry can match even once, no fill possible
        if min_times_matchable == 0 {
            return None;
        }
        
        // Create filled entries
        for (idx, (_lineup_idx, entry)) in best_entries.iter().enumerate() {
            let original_portion = entry.portion;
            let matched_portion = if idx == aggressor_idx {
                entry.portion.saturating_sub(aggressor_portion_adjustment)
            } else {
                entry.portion
            };
            
            // Matched quantity is: (number of times we match) * (matched portion for this entry)
            let matched_quantity = min_times_matchable * matched_portion;
            
            filled_entries.push(FilledEntry {
                entry: entry.clone(),
                original_portion,
                matched_portion,
                matched_quantity,
            });
        }
        
        // Apply the fills - consume quantity from all matched entries
        for fill in &filled_entries {
            if let Some(book_entry) = self.state.books[fill.entry.lineup_index]
                .find_entry_mut(fill.entry.id)
            {
                book_entry.consume_quantity(fill.matched_quantity);
            }
        }
        
        // Remove depleted entries from all books
        for book in &mut self.state.books {
            book.remove_depleted_entries();
        }
        
        Some(FillEvent {
            filled_entries,
            aggressor_lineup_index,
        })
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    
    #[test]
    fn test_basic_limit_entry() {
        let mut pool = EntryPool::new(1000, 2); // 4 lineups
        
        // Submit entries to all 4 lineups
        let submit0 = pool.submit_entry(0, EntryParameters {
            entry_id: 0,
            entry_type: EntryType::Limit,
            portion: 250,
            quantity: 250,
        }).unwrap();
        let submit1 = pool.submit_entry(1, EntryParameters {
            entry_id: 1,
            entry_type: EntryType::Limit,
            portion: 250,
            quantity: 250,
        }).unwrap();
        let submit2 = pool.submit_entry(2, EntryParameters {
            entry_id: 2,
            entry_type: EntryType::Limit,
            portion: 250,
            quantity: 250,
        }).unwrap();
        let submit3 = pool.submit_entry(3, EntryParameters {
            entry_id: 3,
            entry_type: EntryType::Limit,
            portion: 250,
            quantity: 250,
        }).unwrap();
        
        // Last submission should trigger a fill event
        assert_eq!(submit0.fill_events.len(), 0);
        assert_eq!(submit1.fill_events.len(), 0);
        assert_eq!(submit2.fill_events.len(), 0);
        assert_eq!(submit3.fill_events.len(), 1);
        
        // All entries should be consumed in a fill event
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
            quantity: 300,
        }).unwrap();
        pool.submit_entry(1, EntryParameters {
            entry_id: 1,
            entry_type: EntryType::Limit,
            portion: 300,
            quantity: 300,
        }).unwrap();
        pool.submit_entry(2, EntryParameters {
            entry_id: 2,
            entry_type: EntryType::Limit,
            portion: 300,
            quantity: 300,
        }).unwrap();
        
        // Aggressor with portion that would overfill
        let submit = pool.submit_entry(3, EntryParameters {
            entry_id: 3,
            entry_type: EntryType::Limit,
            portion: 200,
            quantity: 200,
        }).unwrap();
        
        // Should have 1 fill event
        assert_eq!(submit.fill_events.len(), 1);
        
        // Aggressor should have portion reduced from 200 to 100
        let fill_event = &submit.fill_events[0];
        let aggressor_entry = fill_event.filled_entries.iter()
            .find(|e| e.entry.lineup_index == 3)
            .unwrap();
        assert_eq!(aggressor_entry.original_portion, 200);
        assert_eq!(aggressor_entry.matched_portion, 100);
        
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
            quantity: 200,
        }).unwrap();
        pool.submit_entry(1, EntryParameters {
            entry_id: 1,
            entry_type: EntryType::Limit,
            portion: 250,
            quantity: 250,
        }).unwrap();
        pool.submit_entry(2, EntryParameters {
            entry_id: 2,
            entry_type: EntryType::Limit,
            portion: 250,
            quantity: 250,
        }).unwrap();
        
        // Market entry should calculate portion = 300
        let submit = pool.submit_entry(3, EntryParameters {
            entry_id: 3,
            entry_type: EntryType::Market,
            portion: 0, // this field should get ignored by submit_entry()
            quantity: 300,
        }).unwrap();
        
        // Should have 1 fill event
        assert_eq!(submit.fill_events.len(), 1);
        assert_eq!(submit.fill_events[0].filled_entries[submit.fill_events[0].aggressor_lineup_index].matched_portion, 300);
        
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
            quantity: 500,
        }).unwrap();
        pool.submit_entry(1, EntryParameters {
            entry_id: 1,
            entry_type: EntryType::Limit,
            portion: 500,
            quantity: 500,
        }).unwrap();
        
        // Market entry should be rejected - missing lineups
        let result = pool.submit_entry(2, EntryParameters {
            entry_id: 2,
            entry_type: EntryType::Market,
            portion: 0,
            quantity: 300,
        });
        assert!(result.is_err());
    }
    
    #[test]
    fn test_multiple_fill_events_from_one_aggressor() {
        let mut pool = EntryPool::new(1000, 2); // 4 lineups
        
        // Submit passive entries with enough quantity for 2 fill events each
        pool.submit_entry(0, EntryParameters {
            entry_id: 0,
            entry_type: EntryType::Limit,
            portion: 250,
            quantity: 500,
        }).unwrap();
        pool.submit_entry(1, EntryParameters {
            entry_id: 1,
            entry_type: EntryType::Limit,
            portion: 250,
            quantity: 500,
        }).unwrap();
        pool.submit_entry(2, EntryParameters {
            entry_id: 2,
            entry_type: EntryType::Limit,
            portion: 250,
            quantity: 250,
        }).unwrap();
        pool.submit_entry(2, EntryParameters {
            entry_id: 3,
            entry_type: EntryType::Limit,
            portion: 250,
            quantity: 250,
        }).unwrap();
        
        // Aggressor with enough quantity for 2 fill events
        let submit = pool.submit_entry(3, EntryParameters {
            entry_id: 4,
            entry_type: EntryType::Limit,
            portion: 250,
            quantity: 500,
        }).unwrap();

        // Should have 2 fill events
        assert_eq!(submit.fill_events.len(), 2);
        
        // All should be consumed in 2 fill events
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
            quantity: 300,
        }).unwrap();
        pool.submit_entry(0, EntryParameters {
            entry_id: 1,
            entry_type: EntryType::Limit,
            portion: 500,
            quantity: 500,
        }).unwrap(); // Largest
        pool.submit_entry(0, EntryParameters {
            entry_id: 2,
            entry_type: EntryType::Limit,
            portion: 400,
            quantity: 400,
        }).unwrap();
        
        // Best entry should be 500
        let state = pool.get_state();
        assert_eq!(state.books[0].best_entry().unwrap().portion, 500);
    }
    
    #[test]
    fn test_quantity_not_multiple_of_portion() {
        let mut pool = EntryPool::new(1000, 1);
        
        let result = pool.submit_entry(0, EntryParameters {
            entry_id: 0,
            entry_type: EntryType::Limit,
            portion: 250,
            quantity: 251,
        });
        assert!(result.is_err());
    }
}
