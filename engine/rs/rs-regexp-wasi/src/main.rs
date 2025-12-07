use std::io;

use io::Read;
use io::Write;

use regex::Regex;

#[derive(serde::Serialize)]
struct PatternTestResult {
    is_match: bool,
    error: String,
}

#[derive(serde::Deserialize)]
struct UntrustedInput {
    pattern: String,
    text: String,
}

impl UntrustedInput {
    fn to_result(&self) -> Result<bool, PatTestErr> {
        let pat = Regex::new(self.pattern.as_str())
            .map_err(|e| PatTestErr::InvalidPattern(format!("invalid regular expression: {e}")))?;

        Ok(pat.is_match(self.text.as_str()))
    }
}

enum PatTestErr {
    InvalidInput(String),
    InvalidPattern(String),
    IoError(io::Error),
}

fn sub() -> Result<bool, PatTestErr> {
    let ijson_max: u64 = 1048576;
    let iraw = io::stdin();
    let i = iraw.lock();
    let mut taken = i.take(ijson_max);
    let mut ijson: Vec<u8> = vec![];
    taken.read_to_end(&mut ijson).map_err(PatTestErr::IoError)?;

    let parsed: UntrustedInput = serde_json::from_slice(&ijson)
        .map_err(|e| PatTestErr::InvalidInput(format!("unable to parse the input json: {e}")))?;

    parsed.to_result()
}

fn main() -> Result<(), io::Error> {
    let imat: Result<bool, PatTestErr> = sub();

    let rslt: PatternTestResult = match imat {
        Ok(b) => Ok(PatternTestResult {
            is_match: b,
            error: "".into(),
        }),
        Err(PatTestErr::IoError(i)) => Err(i),
        Err(PatTestErr::InvalidInput(i)) => Ok(PatternTestResult {
            is_match: false,
            error: i,
        }),
        Err(PatTestErr::InvalidPattern(i)) => Ok(PatternTestResult {
            is_match: false,
            error: i,
        }),
    }?;

    let o = io::stdout();
    let mut ol = o.lock();

    serde_json::to_writer(&mut ol, &rslt)?;

    ol.flush()?;
    Ok(())
}
