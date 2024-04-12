use std::{fmt::Display, ops::Deref, pin::Pin, sync::Arc};

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

impl Display for Repository {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        f.write_fmt(format_args!("{}/{}", self.owner, self.name))
    }
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

#[derive(Clone, Debug, Deserialize)]
pub struct GiteaWebhook {
    id: isize,
    #[serde(rename = "type")]
    r#type: GiteaWebhookType,
    config: GiteaWebhookConfig,
}
#[derive(Clone, Debug, Deserialize)]
pub struct GiteaWebhookConfig {
    url: String,
}

#[derive(Clone, Debug, Deserialize, Serialize, PartialEq, Eq)]
pub enum GiteaWebhookType {
    #[serde(rename = "gitea")]
    Gitea,
    Other(String),
}

#[derive(Clone, Debug, Serialize)]
pub struct CreateGiteaWebhook {
    active: bool,
    authorization_header: Option<String>,
    branch_filter: Option<String>,
    config: CreateGiteaWebhookConfig,
    events: Vec<String>,
    #[serde(rename = "type")]
    r#type: GiteaWebhookType,
}

#[derive(Clone, Debug, Serialize)]
pub struct CreateGiteaWebhookConfig {
    content_type: String,
    url: String,
}

impl DefaultGiteaClient {
    async fn fetch_user_repos_page(
        &self,
        page: usize,
    ) -> anyhow::Result<(Vec<Repository>, Vec<usize>)> {
        let client = reqwest::Client::new();

        let url = format!("{}/api/v1/user/repos?page={page}&limit=50", self.url);

        tracing::trace!("calling url: {}", &url);

        let response = client
            .get(&url)
            .header("Content-Type", "application/json")
            .header("Authorization", format!("token {}", self.token))
            .send()
            .await?;

        let mut pages = Vec::new();
        if page <= 1 {
            if let Some(link_header) = response.headers().get("link") {
                let link_str = link_header.to_str()?;
                pages = parse_link(page, link_str)?;
            }
        }

        let repositories = response.json::<Vec<GiteaRepository>>().await?;

        Ok((
            repositories
                .into_iter()
                .flat_map(Repository::try_from)
                .collect(),
            pages,
        ))
    }

    pub async fn fetch_user_repos(&self) -> anyhow::Result<Vec<Repository>> {
        let (repos, pages) = self.fetch_user_repos_page(1).await?;

        let tasks = pages
            .into_iter()
            .map(|page| async move {
                let (new_repos, _) = self.fetch_user_repos_page(page).await?;

                Ok::<Vec<Repository>, anyhow::Error>(new_repos)
            })
            .collect::<FuturesUnordered<_>>();

        let res: Result<Vec<Vec<Repository>>, anyhow::Error> = tasks.try_collect().await;
        let res = res?.into_iter().flatten();

        Ok(repos.into_iter().chain(res).collect())
    }

    async fn fetch_org_repos_page(
        &self,
        org: &str,
        page: usize,
    ) -> anyhow::Result<(Vec<Repository>, Vec<usize>)> {
        let client = reqwest::Client::new();

        let url = format!(
            "{}/api/v1/orgs/{}/repos?page={page}&limit=50",
            self.url, org
        );

        tracing::trace!("calling url: {}", &url);

        let response = client
            .get(&url)
            .header("Content-Type", "application/json")
            .header("Authorization", format!("token {}", self.token))
            .send()
            .await?;

        let mut pages = Vec::new();
        if page <= 1 {
            if let Some(link_header) = response.headers().get("link") {
                let link_str = link_header.to_str()?;
                pages = parse_link(page, link_str)?;
            }
        }

        let repositories = response.json::<Vec<GiteaRepository>>().await?;

        Ok((
            repositories
                .into_iter()
                .flat_map(Repository::try_from)
                .collect(),
            pages,
        ))
    }

    pub async fn fetch_org_repos(&self, org: &str) -> anyhow::Result<Vec<Repository>> {
        let (repos, pages) = self.fetch_org_repos_page(org, 1).await?;

        let tasks = pages
            .into_iter()
            .map(|page| async move {
                let (new_repos, _) = self.fetch_org_repos_page(org, page).await?;

                Ok::<Vec<Repository>, anyhow::Error>(new_repos)
            })
            .collect::<FuturesUnordered<_>>();

        let res: Result<Vec<Vec<Repository>>, anyhow::Error> = tasks.try_collect().await;
        let res = res?.into_iter().flatten();

        Ok(repos.into_iter().chain(res).collect())
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

    async fn get_webhook(&self, repo: &Repository) -> anyhow::Result<Option<GiteaWebhook>> {
        let client = reqwest::Client::new();

        let url = format!(
            "{}/api/v1/repos/{}/{}/hooks",
            self.url, &repo.owner, &repo.name
        );

        tracing::trace!("calling url: {}", &url);

        let response = client
            .get(&url)
            .header("Content-Type", "application/json")
            .header("Authorization", format!("token {}", self.token))
            .send()
            .await?;

        let webhooks = response.json::<Vec<GiteaWebhook>>().await?;

        let valid_webhooks = webhooks
            .into_iter()
            .filter(|w| w.r#type == GiteaWebhookType::Gitea)
            .filter(|w| w.config.url.contains("contractor"))
            .collect::<Vec<_>>();

        Ok(valid_webhooks.first().map(|f| f.to_owned()))
    }

    async fn add_webhook(&self, repo: &Repository) -> anyhow::Result<()> {
        let client = reqwest::Client::new();

        let url = format!(
            "{}/api/v1/repos/{}/{}/hooks",
            self.url, &repo.owner, &repo.name
        );

        let val = CreateGiteaWebhook {
            active: true,
            authorization_header: Some("something".into()),
            branch_filter: Some("*".into()),
            config: CreateGiteaWebhookConfig {
                content_type: "json".into(),
                url: "https://url?type=contractor".into(),
            },
            events: vec!["pull_request_comment".into(), "issue_comment".into()],
            r#type: GiteaWebhookType::Gitea,
        };

        tracing::trace!(
            "calling url: {} with body {}",
            &url,
            serde_json::to_string(&val)?
        );

        let response = client
            .post(&url)
            .header("Content-Type", "application/json")
            .header("Accept", "application/json")
            .header("Authorization", format!("token {}", self.token))
            .json(&val)
            .send()
            .await?;

        if let Err(e) = response.error_for_status_ref() {
            if let Ok(ok) = response.text().await {
                anyhow::bail!("failed to create webhook: {}, body: {}", e, ok);
            }

            anyhow::bail!("failed to create webhook: {}", e)
        }

        Ok(())
    }

    async fn update_webhook(&self, repo: &Repository, webhook: GiteaWebhook) -> anyhow::Result<()> {
        let client = reqwest::Client::new();

        let url = format!(
            "{}/api/v1/repos/{}/{}/hooks/{}",
            self.url, &repo.owner, &repo.name, &webhook.id,
        );

        let val = CreateGiteaWebhook {
            active: true,
            authorization_header: Some("something".into()),
            branch_filter: Some("*".into()),
            config: CreateGiteaWebhookConfig {
                content_type: "json".into(),
                url: "https://url?type=contractor".into(),
            },
            events: vec!["pull_request_comment".into(), "issue_comment".into()],
            r#type: GiteaWebhookType::Gitea,
        };

        tracing::trace!(
            "calling url: {} with body {}",
            &url,
            serde_json::to_string(&val)?
        );

        let response = client
            .patch(&url)
            .header("Content-Type", "application/json")
            .header("Accept", "application/json")
            .header("Authorization", format!("token {}", self.token))
            .json(&val)
            .send()
            .await?;

        if let Err(e) = response.error_for_status_ref() {
            if let Ok(ok) = response.text().await {
                anyhow::bail!("failed to create webhook: {}, body: {}", e, ok);
            }

            anyhow::bail!("failed to create webhook: {}", e)
        }

        Ok(())
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

    fn ensure_webhook<'a>(
        &'a self,
        repo: &'a Repository,
        force_refresh: bool,
    ) -> Pin<Box<dyn futures::prelude::Future<Output = anyhow::Result<()>> + Send + 'a>> {
        tracing::trace!("ensuring webhook exists for repo: {}", repo);

        Box::pin(async move {
            match (self.get_webhook(repo).await?, force_refresh) {
                (Some(_), false) => {
                    tracing::trace!("webhook already found for {} skipping...", repo);
                }
                (Some(webhook), true) => {
                    tracing::trace!("webhook already found for {} refreshing it", repo);
                    self.update_webhook(repo, webhook).await?;
                }
                (None, _) => {
                    tracing::trace!("webhook was not found for {} adding", repo);
                    self.add_webhook(repo).await?;
                }
            }

            Ok(())
        })
    }
}

// <https://git.front.kjuulh.io/api/v1/user/repos?page=2>; rel="next",<https://git.front.kjuulh.io/api/v1/user/repos?page=9>; rel="last"
fn parse_link(page: usize, link_str: &str) -> anyhow::Result<Vec<usize>> {
    let link_sections = link_str.split(',');

    for link_section in link_sections {
        if let Some((link, rel)) = link_section.rsplit_once("; ") {
            if rel == r#"rel="last""# {
                let actual_link = &link[1..link.len() - 1];

                let url = Url::parse(actual_link)?;

                if let Some(page_num) = url
                    .query_pairs()
                    .into_iter()
                    .find(|(name, _)| name == "page")
                    .map(|(_, value)| value)
                {
                    let page_num: usize = page_num.parse()?;

                    let page_numbers = (page + 1..page_num).collect::<Vec<usize>>();

                    return Ok(page_numbers);
                }
            }
        }
    }

    Ok(Vec::default())
}

mod extensions;
pub mod traits;

use anyhow::Context;
pub use extensions::*;
use futures::{stream::FuturesUnordered, StreamExt, TryStreamExt};
use itertools::Itertools;
use reqwest::{StatusCode, Url};
use serde::{Deserialize, Serialize};
