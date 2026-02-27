pub mod entry_pool;
pub mod math_utils;
pub mod pool_manager;
pub mod pool_utils;

// Common types (from Common.proto with package OpenBook.CommonPackage)
pub mod common {
    tonic::include_proto!("open_book.common_package");
}

// Re-export common types at the matching_service_package level for convenience
pub mod matching_service_package {
    tonic::include_proto!("open_book.matching_service_package");

    // Re-export common types
    pub use super::common::OrderType;
    pub use super::common::SequencedMessageBase;
    pub use super::common::ResponseBase;
    pub use super::common::FallibleBase;
}
