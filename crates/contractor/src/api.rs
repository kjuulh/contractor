use std::net::SocketAddr;

use axum::{extract::MatchedPath, http::Request, routing::get, Router};
use tower_http::trace::TraceLayer;

use crate::SharedState;

pub async fn serve_axum(state: &SharedState, host: &SocketAddr) -> Result<(), anyhow::Error> {
    tracing::info!("running webhook server");
    let app = Router::new()
        .route("/", get(root))
        .with_state(state.clone())
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
