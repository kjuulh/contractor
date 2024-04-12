use crate::SharedState;

use super::GiteaClient;

pub trait GiteaClientState {
    fn gitea_client(&self) -> GiteaClient {
        GiteaClient::new()
    }
}

impl GiteaClientState for SharedState {}
