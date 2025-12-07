fn main() -> Result<(), Box<dyn std::error::Error>> {
    // First compile Common.proto (no package, goes to _.rs)
    tonic_build::configure()
        .compile_protos(&["proto/Common.proto"], &["proto"])?;

    // Then compile MatchingService.proto with extern_path references to common types
    tonic_build::configure()
        .extern_path(".SequencedMessageBase", "crate::common::SequencedMessageBase")
        .extern_path(".ResponseBase", "crate::common::ResponseBase")
        .extern_path(".FallibleBase", "crate::common::FallibleBase")
        .extern_path(".OrderType", "crate::common::OrderType")
        .compile_protos(&["proto/MatchingService.proto"], &["proto"])?;

    Ok(())
}
