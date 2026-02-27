fn main() -> Result<(), Box<dyn std::error::Error>> {
    // First compile Common.proto (package OpenBook.CommonPackage)
    tonic_build::configure()
        .compile_protos(&["../proto/Common.proto"], &["../proto"])?;

    // Then compile MatchingService.proto with extern_path mapping the entire common package
    tonic_build::configure()
        .extern_path(".OpenBook.CommonPackage", "crate::common")
        .compile_protos(&["../proto/MatchingService.proto"], &["../proto"])?;

    Ok(())
}
