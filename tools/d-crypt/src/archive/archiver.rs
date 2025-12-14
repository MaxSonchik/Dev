use std::path::Path; // Убрали File
use flate2::write::GzEncoder;
use flate2::read::GzDecoder;
use flate2::Compression;
use anyhow::{Result, Context};

pub struct Archiver;

impl Archiver {
    pub fn compress_directory(path: &Path) -> Result<Vec<u8>> {
        let mut encoder = GzEncoder::new(Vec::new(), Compression::default());
        {
            let mut tar = tar::Builder::new(&mut encoder);
            tar.append_dir_all(".", path)
                .context("Failed to append directory to archive")?;
            tar.finish().context("Failed to finish tar archive")?;
        }
        encoder.finish().context("Failed to finish gzip compression")
    }

    pub fn decompress_to(data: &[u8], output_path: &Path) -> Result<()> {
        let decoder = GzDecoder::new(data);
        let mut archive = tar::Archive::new(decoder);
        archive.unpack(output_path)
            .context("Failed to unpack archive")?;
        Ok(())
    }
}