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

        tracing::info!("found repositories: {}", repos.len());

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
}

pub trait ReconcilerState {
    fn reconciler(&self) -> Reconciler;
}

impl ReconcilerState for SharedState {
    fn reconciler(&self) -> Reconciler {
        Reconciler::new(self.gitea_client())
    }
}
