use std::{ops::Deref, pin::Pin, sync::Arc};

type DynGiteaClient = Arc<dyn traits::GiteaClient + Send + Sync + 'static>;
pub struct GiteaClient(DynGiteaClient);

impl GiteaClient {
    pub fn new() -> Self {
        Self(Arc::new(DefaultGiteaClient::default()))
    }
}

impl Deref for GiteaClient {
    type Target = DynGiteaClient;

    fn deref(&self) -> &Self::Target {
        &self.0
    }
}

#[derive(Clone, Debug, PartialEq, Eq, Hash)]
pub struct Repository {
    pub owner: String,
    pub name: String,
}

impl TryFrom<GiteaRepository> for Repository {
    type Error = anyhow::Error;

    fn try_from(value: GiteaRepository) -> Result<Self, Self::Error> {
        let (owner, name) = value
            .full_name
            .split_once('/')
            .ok_or(anyhow::anyhow!(
                "name of repository is invalid, should contain a /"
            ))
            .map_err(|e| {
                tracing::warn!("failed to parse repository: {}", e);

                e
            })?;

        Ok(Repository {
            owner: owner.into(),
            name: name.into(),
        })
    }
}

#[derive(Clone, Debug, Deserialize)]
pub struct GiteaRepository {
    full_name: String,
}

pub struct DefaultGiteaClient {
    url: String,
    token: String,
}

impl Default for DefaultGiteaClient {
    fn default() -> Self {
        Self {
            url: std::env::var("GITEA_URL")
                .context("GITEA_URL should be set")
                .map(|g| g.trim_end_matches('/').to_string())
                .unwrap(),
            token: std::env::var("GITEA_TOKEN")
                .context("GITEA_TOKEN should be set")
                .unwrap(),
        }
    }
}

impl DefaultGiteaClient {
    pub async fn fetch_user_repos(&self) -> anyhow::Result<Vec<Repository>> {
        //FIXME: We should collect the pages for these queries
        let client = reqwest::Client::new();

        let url = format!("{}/api/v1/user/repos", self.url);

        tracing::trace!("calling url: {}", &url);

        let response = client
            .get(&url)
            .header("Content-Type", "application/json")
            .header("Authorization", format!("token {}", self.token))
            .send()
            .await?;

        let repositories = response.json::<Vec<GiteaRepository>>().await?;

        Ok(repositories
            .into_iter()
            .flat_map(Repository::try_from)
            .collect())
    }

    pub async fn fetch_org_repos(&self, org: &str) -> anyhow::Result<Vec<Repository>> {
        let client = reqwest::Client::new();

        let url = format!("{}/api/v1/orgs/{}/repos", self.url, org);

        tracing::trace!("calling url: {}", &url);

        let response = client
            .get(&url)
            .header("Content-Type", "application/json")
            .header("Authorization", format!("token {}", self.token))
            .send()
            .await?;

        let repositories = response.json::<Vec<GiteaRepository>>().await?;

        Ok(repositories
            .into_iter()
            .flat_map(Repository::try_from)
            .collect())
    }

    async fn fetch_renovate(&self, repo: &Repository) -> anyhow::Result<Option<()>> {
        let client = reqwest::Client::new();

        let url = format!(
            "{}/api/v1/repos/{}/{}/contents/renovate.json",
            self.url, &repo.owner, &repo.name
        );

        tracing::trace!("calling url: {}", &url);

        let response = client
            .get(&url)
            .header("Content-Type", "application/json")
            .header("Authorization", format!("token {}", self.token))
            .send()
            .await?;

        match response.error_for_status() {
            Ok(_) => Ok(Some(())),
            Err(e) => match e.status() {
                Some(StatusCode::NOT_FOUND) => Ok(None),
                _ => anyhow::bail!(e),
            },
        }
    }
}

impl traits::GiteaClient for DefaultGiteaClient {
    fn get_user_repositories<'a>(
        &'a self,
        user: &str,
    ) -> Pin<Box<dyn futures::prelude::Future<Output = anyhow::Result<Vec<Repository>>> + Send + 'a>>
    {
        tracing::debug!("fetching gitea repositories for user: {user}");

        Box::pin(async { self.fetch_user_repos().await })
    }

    fn get_org_repositories<'a>(
        &'a self,
        org: &'a str,
    ) -> Pin<Box<dyn futures::prelude::Future<Output = anyhow::Result<Vec<Repository>>> + Send + 'a>>
    {
        tracing::debug!("fetching gitea repositories for org: {org}");

        Box::pin(async move { self.fetch_org_repos(org).await })
    }

    fn renovate_enabled<'a>(
        &'a self,
        repo: &'a Repository,
    ) -> Pin<Box<dyn futures::prelude::Future<Output = anyhow::Result<bool>> + Send + 'a>> {
        tracing::trace!("checking whether renovate is enabled for: {:?}", repo);

        Box::pin(async { self.fetch_renovate(repo).await.map(|s| s.is_some()) })
    }
}

mod extensions;
pub mod traits;

use anyhow::Context;
pub use extensions::*;
use reqwest::StatusCode;
use serde::Deserialize;
