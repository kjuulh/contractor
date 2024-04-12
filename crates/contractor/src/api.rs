use std::{net::SocketAddr, sync::Arc};

use anyhow::Context;
use axum::{
    body::Body,
    extract::{MatchedPath, State},
    http::Request,
    response::IntoResponse,
    routing::{get, post},
    Json, Router,
};
use serde::{Deserialize, Serialize};
use tower_http::trace::TraceLayer;

use crate::{
    services::{
        bot::{BotRequest, BotState},
        gitea::Repository,
    },
    SharedState,
};

pub async fn serve_axum(state: &SharedState, host: &SocketAddr) -> Result<(), anyhow::Error> {
    tracing::info!("running webhook server");
    let app = Router::new()
        .route("/", get(root))
        .route("/webhooks/gitea", post(gitea_webhook))
        .with_state(state.to_owned())
        .layer(
            TraceLayer::new_for_http().make_span_with(|request: &Request<_>| {
                // Log the matched route's path (with placeholders not filled in).
                // Use request.uri() or OriginalUri if you want the real path.
                let matched_path = request
                    .extensions()
                    .get::<MatchedPath>()
                    .map(MatchedPath::as_str);

                tracing::info_span!(
                    "http_request",
                    method = ?request.method(),
                    matched_path,
                    some_other_field = tracing::field::Empty,
                )
            }), // ...
        );

    tracing::info!("listening on {}", host);
    let listener = tokio::net::TcpListener::bind(host).await.unwrap();
    axum::serve(listener, app.into_make_service()).await?;

    Ok(())
}

async fn root() -> &'static str {
    "Hello, contractor!"
}

#[derive(Serialize, Deserialize, Clone, Debug)]
pub struct GiteaWebhookComment {
    body: String,
}
#[derive(Serialize, Deserialize, Clone, Debug)]
pub struct GiteaWebhookRepository {
    full_name: String,
}

#[derive(Serialize, Deserialize, Clone, Debug)]
#[serde(untagged)]
pub enum GiteaWebhook {
    Issue {
        comment: GiteaWebhookComment,
        repository: GiteaWebhookRepository,
    },
}

pub enum ApiError {
    InternalError(anyhow::Error),
}

impl IntoResponse for ApiError {
    fn into_response(self) -> axum::response::Response {
        match self {
            ApiError::InternalError(e) => {
                tracing::error!("failed with internal error: {}", e);

                (axum::http::StatusCode::INTERNAL_SERVER_ERROR, e.to_string())
            }
        }
        .into_response()
    }
}

async fn gitea_webhook(
    State(state): State<SharedState>,
    Json(json): Json<GiteaWebhook>,
) -> Result<impl IntoResponse, ApiError> {
    tracing::info!(
        "called: {}",
        serde_json::to_string(&json)
            .context("failed to serialize webhook")
            .map_err(ApiError::InternalError)?
    );

    let bot_req: BotRequest = json.try_into().map_err(ApiError::InternalError)?;

    state
        .bot()
        .handle_request(bot_req)
        .await
        .map_err(ApiError::InternalError)?;

    Ok("Hello, contractor!")
}

impl TryFrom<GiteaWebhook> for BotRequest {
    type Error = anyhow::Error;
    fn try_from(value: GiteaWebhook) -> Result<Self, Self::Error> {
        match value {
            GiteaWebhook::Issue {
                comment,
                repository,
            } => {
                let (owner, name) = repository.full_name.split_once('/').ok_or(anyhow::anyhow!(
                    "{} did not contain a valid owner/repository",
                    &repository.full_name
                ))?;

                Ok(BotRequest {
                    repo: Repository {
                        owner: owner.into(),
                        name: name.into(),
                    },
                    command: comment.body,
                })
            }
        }
    }
}
