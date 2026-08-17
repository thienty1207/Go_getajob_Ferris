use anyhow::{anyhow, Context, Result};
use std::{collections::HashMap, env, path::Path};
use url::Url;

pub const DEFAULT_CRAWL_INTERVAL_SECONDS: u64 = 6 * 60 * 60;
pub const MIN_CRAWL_INTERVAL_SECONDS: u64 = 15 * 60;
pub const MAX_CRAWL_INTERVAL_SECONDS: u64 = 7 * 24 * 60 * 60;

pub fn load_database_url() -> Result<String> {
    let mut values = HashMap::new();
    let local_env = Path::new(".env");
    let crawler_env = Path::new(env!("CARGO_MANIFEST_DIR")).join(".env");
    load_dotenv_values(local_env, &mut values);
    load_dotenv_values(&crawler_env, &mut values);
    for key in [
        "DATABASE_URL",
        "DATABASE_USER",
        "DB_USER",
        "PGUSER",
        "DATABASE_PASSWORD",
        "DB_PASSWORD",
        "PGPASSWORD",
        "DATABASE_HOST",
        "DB_HOST",
        "PGHOST",
        "DATABASE_PORT",
        "DB_PORT",
        "PGPORT",
        "DATABASE_NAME",
        "DB_NAME",
        "PGDATABASE",
    ] {
        if let Ok(value) = env::var(key) {
            values.insert(key.to_owned(), value);
        }
    }
    build_database_url_from(&values)
}

pub fn load_crawl_interval_seconds() -> Result<u64> {
    let mut values = HashMap::new();
    let local_env = Path::new(".env");
    let crawler_env = Path::new(env!("CARGO_MANIFEST_DIR")).join(".env");
    load_dotenv_values(local_env, &mut values);
    load_dotenv_values(&crawler_env, &mut values);
    if let Ok(value) = env::var("CRAWL_INTERVAL_SECONDS") {
        values.insert("CRAWL_INTERVAL_SECONDS".to_owned(), value);
    }
    crawl_interval_seconds_from(&values)
}

fn crawl_interval_seconds_from(values: &HashMap<String, String>) -> Result<u64> {
    let raw = value(values, &["CRAWL_INTERVAL_SECONDS"])
        .unwrap_or_else(|| DEFAULT_CRAWL_INTERVAL_SECONDS.to_string());
    database_crawl_interval_seconds(&raw)
}

pub fn database_crawl_interval_seconds(raw: &str) -> Result<u64> {
    let seconds = raw
        .trim()
        .parse::<u64>()
        .map_err(|_| anyhow!("crawler interval setting must be an integer"))?;
    if !(MIN_CRAWL_INTERVAL_SECONDS..=MAX_CRAWL_INTERVAL_SECONDS).contains(&seconds) {
        return Err(anyhow!(
            "crawler interval setting must be between {MIN_CRAWL_INTERVAL_SECONDS} and {MAX_CRAWL_INTERVAL_SECONDS} seconds"
        ));
    }
    Ok(seconds)
}

fn load_dotenv_values(path: &Path, values: &mut HashMap<String, String>) {
    let Ok(entries) = dotenvy::from_path_iter(path) else {
        return;
    };
    for entry in entries.flatten() {
        values.insert(entry.0, entry.1);
    }
}

pub fn build_database_url_from(values: &HashMap<String, String>) -> Result<String> {
    if let Some(database_url) = value(values, &["DATABASE_URL"]) {
        return Ok(database_url);
    }

    let username = value(
        values,
        &[
            "DATABASE_USER",
            "DB_USER",
            "PGUSER",
            "username",
            "USERNAME",
            "user",
        ],
    )
    .context("DATABASE user is required for crawler")?;
    let password = value(
        values,
        &[
            "DATABASE_PASSWORD",
            "DB_PASSWORD",
            "PGPASSWORD",
            "password",
            "PASSWORD",
        ],
    )
    .unwrap_or_default();
    let host = value(
        values,
        &["DATABASE_HOST", "DB_HOST", "PGHOST", "host", "HOST"],
    )
    .unwrap_or_else(|| "127.0.0.1".to_owned());
    let port = value(
        values,
        &["DATABASE_PORT", "DB_PORT", "PGPORT", "port", "PORT"],
    )
    .unwrap_or_else(|| "5432".to_owned());
    let port = port
        .parse::<u16>()
        .map_err(|_| anyhow!("crawler database port is invalid"))?;
    let database = value(
        values,
        &[
            "DATABASE_NAME",
            "DB_NAME",
            "PGDATABASE",
            "database",
            "DATABASE",
        ],
    )
    .unwrap_or_else(|| "gogetsomefoodferris".to_owned());

    let mut url = Url::parse("postgresql://127.0.0.1").expect("static PostgreSQL URL is valid");
    url.set_username(&username)
        .map_err(|_| anyhow!("crawler database username is invalid"))?;
    url.set_password(Some(&password))
        .map_err(|_| anyhow!("crawler database password is invalid"))?;
    url.set_host(Some(&host))
        .map_err(|_| anyhow!("crawler database host is invalid"))?;
    url.set_port(Some(port))
        .map_err(|_| anyhow!("crawler database port is invalid"))?;
    url.set_path(&format!("/{database}"));
    url.set_query(Some("sslmode=disable"));
    Ok(url.to_string())
}

fn value(values: &HashMap<String, String>, keys: &[&str]) -> Option<String> {
    keys.iter()
        .filter_map(|key| values.get(*key))
        .map(|value| value.trim())
        .find(|value| !value.is_empty())
        .map(ToOwned::to_owned)
}

#[cfg(test)]
mod tests {
    use super::{
        build_database_url_from, crawl_interval_seconds_from, database_crawl_interval_seconds,
    };
    use std::collections::HashMap;

    #[test]
    fn explicit_database_url_wins_over_local_aliases() {
        let values = HashMap::from([
            (
                "DATABASE_URL".to_owned(),
                "postgres://remote.example/db".to_owned(),
            ),
            ("username".to_owned(), "postgres".to_owned()),
            ("password".to_owned(), "secret".to_owned()),
        ]);
        assert_eq!(
            build_database_url_from(&values).unwrap(),
            "postgres://remote.example/db"
        );
    }

    #[test]
    fn local_aliases_build_a_safe_postgres_url() {
        let values = HashMap::from([
            ("username".to_owned(), "post@gres".to_owned()),
            ("password".to_owned(), "p@ss/word".to_owned()),
            ("port".to_owned(), "5432".to_owned()),
            ("database".to_owned(), "ferris jobs".to_owned()),
        ]);
        let url = build_database_url_from(&values).unwrap();
        assert!(
            url.starts_with("postgresql://post%40gres:p%40ss%2Fword@127.0.0.1:5432/ferris%20jobs")
        );
    }

    #[test]
    fn lowercase_local_username_beats_a_host_account_variable() {
        let values = HashMap::from([
            ("USERNAME".to_owned(), "windows-user".to_owned()),
            ("username".to_owned(), "postgres".to_owned()),
        ]);
        assert!(build_database_url_from(&values)
            .unwrap()
            .starts_with("postgresql://postgres@127.0.0.1:5432/"));
    }

    #[test]
    fn crawl_interval_defaults_to_six_hours() {
        assert_eq!(
            crawl_interval_seconds_from(&HashMap::new()).unwrap(),
            21_600
        );
    }

    #[test]
    fn database_interval_value_is_bounded_and_numeric() {
        assert_eq!(database_crawl_interval_seconds("23400").unwrap(), 23_400);
        for value in ["0", "899", "604801", "not-a-number"] {
            assert!(
                database_crawl_interval_seconds(value).is_err(),
                "value {value}"
            );
        }
    }

    #[test]
    fn crawl_interval_accepts_a_bounded_operator_value() {
        let values = HashMap::from([("CRAWL_INTERVAL_SECONDS".to_owned(), "900".to_owned())]);
        assert_eq!(crawl_interval_seconds_from(&values).unwrap(), 900);
    }

    #[test]
    fn crawl_interval_rejects_too_small_or_invalid_values() {
        for value in ["0", "59", "not-a-number"] {
            let values = HashMap::from([("CRAWL_INTERVAL_SECONDS".to_owned(), value.to_owned())]);
            assert!(
                crawl_interval_seconds_from(&values).is_err(),
                "value {value}"
            );
        }
    }
}
