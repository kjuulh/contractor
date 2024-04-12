use crate::{services::gitea::GiteaClientState, SharedState};

pub async fn serve_cron_jobs(state: &SharedState) -> Result<(), anyhow::Error> {
    let state = state.clone();
    tokio::spawn(async move {
        let gitea_client = state.gitea_client();
        loop {
            tracing::info!("running cronjobs");

            tokio::time::sleep(std::time::Duration::from_secs(10_000)).await;
        }
        Ok::<(), anyhow::Error>(())
    })
    .await??;

    Ok(())
}
