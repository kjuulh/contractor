use std::{ops::Deref, sync::Arc};

use crate::services::engines::dagger::Dagger;

#[derive(Clone)]
pub struct SharedState(Arc<State>);

impl From<Arc<State>> for SharedState {
    fn from(value: Arc<State>) -> Self {
        Self(value)
    }
}

impl Deref for SharedState {
    type Target = Arc<State>;

    fn deref(&self) -> &Self::Target {
        &self.0
    }
}

pub struct State {
    // pub db: Pool<Postgres>,
    pub engine: Dagger,
}

impl State {
    pub async fn new() -> anyhow::Result<Self> {
        // let db = sqlx::PgPool::connect(
        //     &std::env::var("DATABASE_URL").context("DATABASE_URL is not set")?,
        // )
        // .await?;

        // sqlx::migrate!("migrations/crdb")
        //     .set_locking(false)
        //     .run(&db)
        //     .await?;

        // let _ = sqlx::query("SELECT 1;").fetch_one(&db).await?;

        // Ok(Self { db })
        let engine = Dagger::new();

        Ok(Self { engine })
    }
}
