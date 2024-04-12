use crate::SharedState;

pub async fn serve_cron_jobs(_state: &SharedState) -> Result<(), anyhow::Error> {
    tokio::spawn(async move {
        loop {
            tracing::info!("running cronjobs");
            tokio::time::sleep(std::time::Duration::from_secs(10_000)).await;
        }
        Ok::<(), anyhow::Error>(())
    })
    .await??;

    Ok(())
}
