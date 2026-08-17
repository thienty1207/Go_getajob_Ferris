use thiserror::Error;
use unicode_normalization::{char::is_combining_mark, UnicodeNormalization};

const MAX_LOCATION_TEXT_CHARS: usize = 512;

#[derive(Debug, Error, PartialEq, Eq)]
pub enum LocationError {
    #[error("location text is blank or too long")]
    InvalidText,
}

pub fn normalize_location_text(value: &str) -> Result<String, LocationError> {
    let value = value.trim();
    if value.is_empty() || value.chars().count() > MAX_LOCATION_TEXT_CHARS {
        return Err(LocationError::InvalidText);
    }

    let value = value.replace(['Đ', 'đ'], "d");
    let mut normalized = String::new();
    let mut previous_space = true;
    for character in value.nfkd().flat_map(char::to_lowercase) {
        if is_combining_mark(character) {
            continue;
        }
        if character.is_alphanumeric() {
            normalized.push(character);
            previous_space = false;
        } else if !previous_space {
            normalized.push(' ');
            previous_space = true;
        }
    }
    let normalized = normalized.trim().to_owned();
    if normalized.is_empty() {
        return Err(LocationError::InvalidText);
    }
    Ok(normalized)
}

#[cfg(test)]
mod tests {
    use super::normalize_location_text;

    #[test]
    fn normalizes_vietnamese_location_variants_to_stable_keys() {
        let cases = [
            ("TP.HCM", "tp hcm"),
            ("Ho Chi Minh City", "ho chi minh city"),
            ("Q.1, TP.HCM", "q 1 tp hcm"),
            ("Hà Nội", "ha noi"),
            ("Đà Nẵng / Việt Nam", "da nang viet nam"),
        ];
        for (input, expected) in cases {
            assert_eq!(normalize_location_text(input).unwrap(), expected);
        }
    }

    #[test]
    fn rejects_blank_and_oversized_location_text() {
        assert!(normalize_location_text("   ").is_err());
        assert!(normalize_location_text(&"x".repeat(513)).is_err());
    }
}
