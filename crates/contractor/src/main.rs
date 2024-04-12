use std::{net::SocketAddr, sync::Arc};

use clap::{Parser, Subcommand};
use futures::{stream::FuturesUnordered, StreamExt};
use tokio::task;

#[derive(Parser)]
#[command(author, version, about, long_about = None, subcommand_required = true)]
struct Command {
    #[command(subcommand)]
    command: Option<Commands>,
}

#[derive(Subcommand)]
enum Commands {
    Serve {
        #[arg(env = "SERVICE_HOST", long, default_value = "127.0.0.1:3000")]
        host: SocketAddr,
    },
}

mod api;
mod schedule;

#[tokio::main]
async fn main() -> anyhow::Result<()> {
    dotenv::dotenv().ok();
    tracing_subscriber::fmt::init();

    let cli = Command::parse();

    if let Some(Commands::Serve { host }) = cli.command {
        tracing::info!("Starting service");

        let state = SharedState::from(Arc::new(State::new().await?));

        let mut tasks = FuturesUnordered::new();

        tasks.push({
            let state = state.clone();
            task::spawn(async move {
                serve_axum(&state, &host).await?;
                Ok::<(), anyhow::Error>(())
            })
        });

        tasks.push(task::spawn(async move {
            serve_cron_jobs(&state).await?;
            Ok::<(), anyhow::Error>(())
        }));

        while let Some(result) = tasks.next().await {
            result??
        }
    }

    Ok(())
}

mod state;
pub use crate::state::{SharedState, State};
use crate::{api::serve_axum, schedule::serve_cron_jobs};
