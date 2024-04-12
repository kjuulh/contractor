use clap::{Parser, Subcommand};

use crate::SharedState;

use super::gitea::Repository;

pub struct Bot {
    command_name: String,
}

#[derive(Parser)]
#[command(author, version, about, long_about = None, subcommand_required = true)]
struct BotCommand {
    #[command(subcommand)]
    command: Option<BotCommands>,
}

#[derive(Subcommand)]
enum BotCommands {
    Refresh {
        #[arg(long)]
        all: bool,
    },
}

impl Bot {
    pub fn new() -> Self {
        Self {
            command_name: std::env::var("CONTRACTOR_COMMAND_NAME").unwrap_or("contractor".into()),
        }
    }

    pub async fn handle_request(&self, req: impl Into<BotRequest>) -> anyhow::Result<()> {
        let req: BotRequest = req.into();

        if !req.command.starts_with(&self.command_name) {
            return Ok(());
        }

        let cmd = BotCommand::parse_from(req.command.split_whitespace());

        match cmd.command {
            Some(BotCommands::Refresh { all }) => {
                tracing::info!("triggering refresh for: {}, all: {}", req.repo, all);
            }
            None => {
                // TODO: Send back the help menu
            }
        }

        Ok(())
    }
}

pub struct BotRequest {
    pub repo: Repository,
    pub command: String,
}

pub trait BotState {
    fn bot(&self) -> Bot {
        Bot::new()
    }
}
impl BotState for SharedState {}
