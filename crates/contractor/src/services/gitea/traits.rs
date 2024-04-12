use std::pin::Pin;

use futures::Future;

use super::Repository;

pub trait GiteaClient {
    fn get_user_repositories<'a>(
        &'a self,
        user: &str,
    ) -> Pin<Box<dyn Future<Output = anyhow::Result<Vec<Repository>>> + Send + 'a>>;

    fn get_org_repositories<'a>(
        &'a self,
        org: &'a str,
    ) -> Pin<Box<dyn Future<Output = anyhow::Result<Vec<Repository>>> + Send + 'a>>;

    fn renovate_enabled<'a>(
        &'a self,
        repo: &'a Repository,
    ) -> Pin<Box<dyn Future<Output = anyhow::Result<bool>> + Send + 'a>>;

    fn ensure_webhook<'a>(
        &'a self,
        repo: &'a Repository,
        force_refresh: bool,
    ) -> Pin<Box<dyn Future<Output = anyhow::Result<()>> + Send + 'a>>;
}
