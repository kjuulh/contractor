use futures::{stream::FuturesUnordered, StreamExt};
use itertools::Itertools;

use crate::SharedState;

use super::gitea::{GiteaClient, GiteaClientState, Repository};

pub struct Reconciler {
    gitea_client: GiteaClient,
}

impl Reconciler {
    pub fn new(gitea_client: GiteaClient) -> Self {
        Self { gitea_client }
    }

    pub async fn reconcile(
        &self,
        user: Option<String>,
        orgs: Option<Vec<String>>,
    ) -> anyhow::Result<()> {
        let repos = self.get_repos(user, orgs).await?;
        tracing::debug!("found repositories: {}", repos.len());

        let renovate_enabled = self.get_renovate_enabled(&repos).await?;
        tracing::debug!(
            "found repositories with renovate enabled: {}",
            renovate_enabled.len()
        );

        Ok(())
    }

    async fn get_repos(
        &self,
        user: Option<String>,
        orgs: Option<Vec<String>>,
    ) -> anyhow::Result<Vec<Repository>> {
        let mut repos = Vec::new();

        if let Some(user) = user {
            let mut r = self.gitea_client.get_user_repositories(&user).await?;

            repos.append(&mut r);
        }

        if let Some(orgs) = orgs {
            for org in orgs {
                let mut r = self.gitea_client.get_org_repositories(&org).await?;
                repos.append(&mut r);
            }
        }

        Ok(repos.into_iter().unique().collect())
    }

    async fn get_renovate_enabled(&self, repos: &[Repository]) -> anyhow::Result<Vec<Repository>> {
        let mut futures = FuturesUnordered::new();

        for repo in repos {
            futures.push(async move {
                let enabled = self.gitea_client.renovate_enabled(repo).await?;

                if enabled {
                    Ok::<Option<Repository>, anyhow::Error>(Some(repo.to_owned()))
                } else {
                    tracing::trace!("repository: {:?}, doesn't have renovate enabled", repo);
                    Ok(None)
                }
            })
        }

        let mut enabled = Vec::new();
        while let Some(res) = futures.next().await {
            let res = res?;

            if let Some(repo) = res {
                enabled.push(repo)
            }
        }

        Ok(enabled)
    }
}

pub trait ReconcilerState {
    fn reconciler(&self) -> Reconciler;
}

impl ReconcilerState for SharedState {
    fn reconciler(&self) -> Reconciler {
        Reconciler::new(self.gitea_client())
    }
}
