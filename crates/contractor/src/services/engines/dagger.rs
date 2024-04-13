use std::{str::FromStr, sync::Arc};

use dagger_sdk::ContainerWithNewFileOptsBuilder;
use futures::Future;
use tokio::sync::RwLock;

type DynDagger = Arc<dyn traits::Dagger + Send + Sync + 'static>;

#[derive(Clone)]
pub struct Dagger {
    dagger: DynDagger,
}

impl Default for Dagger {
    fn default() -> Self {
        Self::new()
    }
}

impl Dagger {
    pub fn new() -> Self {
        Self {
            dagger: Arc::new(DefaultDagger::new()),
        }
    }
}

impl std::ops::Deref for Dagger {
    type Target = DynDagger;

    fn deref(&self) -> &Self::Target {
        &self.dagger
    }
}

struct DefaultDagger {
    client: Arc<RwLock<Option<dagger_sdk::Query>>>,
}

impl DefaultDagger {
    pub fn new() -> Self {
        let client = Arc::new(RwLock::new(None));

        let host =
            std::env::var("CONTRACTOR_DOCKER_HOST").expect("CONTRACTOR_DOCKER_HOST to be set");

        std::env::set_var("DOCKER_HOST", host);

        tokio::spawn({
            let client = client.clone();

            async move {
                let mut client = client.write().await;

                match dagger_sdk::connect().await {
                    Ok(o) => *client = Some(o),
                    Err(e) => tracing::error!("failed to start dagger engine: {}", e),
                };
            }
        });

        Self { client }
    }

    pub async fn get_client(&self) -> dagger_sdk::Query {
        let client = self.client.clone().read().await.clone();

        client.unwrap()
    }
}

impl traits::Dagger for DefaultDagger {
    fn execute_renovate<'a>(
        &'a self,
        config: &'a crate::services::renovate::RenovateConfig,
    ) -> std::pin::Pin<Box<dyn Future<Output = anyhow::Result<()>> + Send + 'a>> {
        Box::pin(async move {
            let renovate_image = "renovate/renovate:37";

            let client = self.get_client().await;

            let github_com_token = client.set_secret(
                "GITHUB_COM_TOKEN",
                std::env::var("CONTRACTOR_GITHUB_COM_TOKEN")
                    .expect("CONTRACTOR_GITHUB_COM_TOKEN to be set"),
            );

            let renovate_secrets = client.set_secret(
                "RENOVATE_SECRETS",
                std::env::var("CONTRACTOR_RENOVATE_SECRETS")
                    .expect("CONTRACTOR_RENOVATE_SECRETS to be set"),
            );

            let renovate_token = client.set_secret(
                "RENOVATE_TOKEN",
                std::env::var("CONTRACTOR_RENOVATE_TOKEN")
                    .expect("CONTRACTOR_RENOVATE_TOKEN to be set"),
            );

            let renovate_file_url = std::env::var("CONTRACTOR_RENOVATE_CONFIG_URL")
                .expect("CONTRACTOR_RENOVATE_CONFIG_URL to be set");

            let renovate_file = client.http(renovate_file_url).contents().await?;

            let mut renovate_file_value: serde_json::Value = serde_json::from_str(&renovate_file)?;
            let obj = renovate_file_value
                .as_object_mut()
                .ok_or(anyhow::anyhow!("config is not a valid json object"))?;

            let _ = obj.insert("autodiscover".into(), serde_json::Value::from_str("false")?);

            let renovate_file = serde_json::to_string(&obj)?;

            let output = client
                .container()
                .from(renovate_image)
                .with_secret_variable("GITHUB_COM_TOKEN", github_com_token)
                .with_secret_variable("RENOVATE_SECRETS", renovate_secrets)
                .with_secret_variable("RENOVATE_TOKEN", renovate_token)
                .with_env_variable("LOG_LEVEL", "info")
                .with_env_variable("RENOVATE_CONFIG_FILE", "/opt/renovate/config.json")
                .with_new_file_opts(
                    "/opt/renovate/config.json",
                    ContainerWithNewFileOptsBuilder::default()
                        .contents(renovate_file.as_str())
                        .permissions(0o644isize)
                        .build()?,
                )
                .with_exec(vec![&config.repo])
                .stdout()
                .await?;

            tracing::debug!(
                "renovate on: {} finished with output {}",
                &config.repo,
                &output
            );

            Ok::<(), anyhow::Error>(())
        })
    }
}

pub mod traits {
    use std::pin::Pin;

    use futures::Future;

    use crate::services::renovate::RenovateConfig;

    pub trait Dagger {
        fn execute_renovate<'a>(
            &'a self,
            config: &'a RenovateConfig,
        ) -> Pin<Box<dyn Future<Output = anyhow::Result<()>> + Send + 'a>>;
    }
}
