use std::path::PathBuf;
use std::sync::Arc;

use clap::Args;
use clap::Parser;
use clap::Subcommand;
use clap::ValueEnum;

use dagger_sdk::Platform;
use dagger_sdk::QueryContainerOpts;

use crate::please_release::run_release_please;

#[derive(Parser, Clone)]
#[command(author, version, about, long_about = None, subcommand_required = true)]
pub struct Command {
    #[command(subcommand)]
    commands: Commands,

    #[command(flatten)]
    global: GlobalArgs,
}

#[derive(Subcommand, Clone)]
pub enum Commands {
    PullRequest {
        #[arg(long)]
        image: String,
        #[arg(long)]
        tag: String,
        #[arg(long)]
        bin_name: String,
    },
    Main {
        #[arg(long)]
        image: String,
        #[arg(long)]
        tag: String,
        #[arg(long)]
        bin_name: String,
    },
    Release,
}

#[derive(Subcommand, Clone)]
pub enum LocalCommands {
    Build {
        #[arg(long, default_value = "debug")]
        profile: BuildProfile,
        #[arg(long)]
        bin_name: String,
    },
    Test,
    DockerImage {
        #[arg(long)]
        image: String,
        #[arg(long)]
        tag: String,
        #[arg(long)]
        bin_name: String,
    },
    PleaseRelease,
    BuildDocs {},
}

#[derive(Debug, Clone, ValueEnum)]
pub enum BuildProfile {
    Debug,
    Release,
}

#[derive(Debug, Clone, Args)]
pub struct GlobalArgs {
    #[arg(long, global = true, help_heading = "Global")]
    dry_run: bool,

    #[arg(long, global = true, help_heading = "Global")]
    golang_builder_image: Option<String>,

    #[arg(long, global = true, help_heading = "Global")]
    production_image: Option<String>,

    #[arg(long, global = true, help_heading = "Global")]
    docker_image: Option<String>,

    #[arg(long, global = true, help_heading = "Global")]
    source: Option<PathBuf>,

    #[arg(long, global = true, help_heading = "Global")]
    docs_image: Option<String>,

    #[arg(long, global = true, help_heading = "Global")]
    docs_image_tag: Option<String>,
}

#[tokio::main]
async fn main() -> eyre::Result<()> {
    let _ = dotenv::dotenv();
    let _ = color_eyre::install();

    let client = dagger_sdk::connect().await?;

    let cli = Command::parse();

    match &cli.commands {
        Commands::PullRequest {
            image,
            tag,
            bin_name,
        } => {
            async fn test(client: Arc<dagger_sdk::Query>, cli: &Command, bin_name: &String) {
                let args = &cli.global;

                let base_image = base_golang_image(client.clone(), args, &None, bin_name)
                    .await
                    .unwrap();
                test::execute(client.clone(), args, base_image)
                    .await
                    .unwrap();
            }
            async fn build(
                client: Arc<dagger_sdk::Query>,
                cli: &Command,
                bin_name: &String,
                image: &String,
                tag: &String,
            ) {
                let args = &cli.global;

                build::build(client.clone(), args, bin_name, image, tag)
                    .await
                    .unwrap();
            }

            tokio::join!(
                test(client.clone(), &cli, bin_name),
                build(client.clone(), &cli, bin_name, image, tag),
            );
        }
        Commands::Main {
            image,
            tag,
            bin_name,
        } => {
            async fn test(client: Arc<dagger_sdk::Query>, cli: &Command, bin_name: &String) {
                let args = &cli.global;

                let base_image = base_golang_image(client.clone(), args, &None, bin_name)
                    .await
                    .unwrap();
                test::execute(client.clone(), args, base_image)
                    .await
                    .unwrap();
            }
            async fn build(
                client: Arc<dagger_sdk::Query>,
                cli: &Command,
                bin_name: &String,
                image: &String,
                tag: &String,
            ) {
                let args = &cli.global;

                build::build_and_deploy(client.clone(), args, bin_name, image, tag)
                    .await
                    .unwrap();
            }

            async fn cuddle_please(client: Arc<dagger_sdk::Query>, cli: &Command) {
                run_release_please(client.clone(), &cli.global)
                    .await
                    .unwrap();
            }

            tokio::join!(
                test(client.clone(), &cli, bin_name),
                build(client.clone(), &cli, bin_name, image, tag),
                cuddle_please(client.clone(), &cli)
            );
        }
        Commands::Release => todo!(),
    }

    Ok(())
}

mod please_release {
    use std::sync::Arc;

    use crate::GlobalArgs;

    pub async fn run_release_please(
        client: Arc<dagger_sdk::Query>,
        _args: &GlobalArgs,
    ) -> eyre::Result<()> {
        let build_image = client
            .container()
            .from("kasperhermansen/cuddle-please:main-1691463075");

        let src = client
            .git_opts(
                "https://git.front.kjuulh.io/kjuulh/contractor",
                dagger_sdk::QueryGitOpts {
                    experimental_service_host: None,
                    keep_git_dir: Some(true),
                },
            )
            .branch("main")
            .tree();

        let res = build_image
            .with_secret_variable(
                "CUDDLE_PLEASE_TOKEN",
                client
                    .set_secret("CUDDLE_PLEASE_TOKEN", std::env::var("CUDDLE_PLEASE_TOKEN")?)
                    .id()
                    .await?,
            )
            .with_workdir("/mnt/app")
            .with_directory(".", src.id().await?)
            .with_exec(vec![
                "git",
                "remote",
                "set-url",
                "origin",
                &format!(
                    "https://git:{}@git.front.kjuulh.io/kjuulh/contractor.git",
                    std::env::var("CUDDLE_PLEASE_TOKEN")?
                ),
            ])
            .with_exec(vec![
                "cuddle-please",
                "release",
                "--engine=gitea",
                "--owner=kjuulh",
                "--repo=contractor",
                "--branch=main",
                "--api-url=https://git.front.kjuulh.io",
                "--log-level=debug",
            ]);

        let exit_code = res.exit_code().await?;
        if exit_code != 0 {
            eyre::bail!("failed to run cuddle-please");
        }

        let please_out = res.stdout().await?;
        println!("{please_out}");
        let please_out = res.stderr().await?;
        println!("{please_out}");

        Ok(())
    }
}

mod build {
    use std::sync::Arc;

    use dagger_sdk::Container;

    use crate::{base_golang_image, get_base_debian_image, GlobalArgs};

    pub async fn build_and_deploy(
        client: Arc<dagger_sdk::Query>,
        args: &GlobalArgs,
        bin_name: &String,
        image: &String,
        tag: &String,
    ) -> eyre::Result<()> {
        // let containers = vec!["linux/amd64", "linux/arm64"];

        let base_image = get_base_debian_image(
            client.clone(),
            &args.clone(),
            Some("linux/amd64".to_string()),
        )
        .await?;

        let container = base_golang_image(
            client.clone(),
            args,
            &Some("linux/amd64".to_string()),
            &bin_name.clone(),
        )
        .await?;
        let build_image = execute(client.clone(), args, &container, &base_image, bin_name).await?;

        let build_id = build_image.id().await?;

        let _container = client
            .clone()
            .container()
            .publish_opts(
                format!("{image}:{tag}"),
                dagger_sdk::ContainerPublishOpts {
                    platform_variants: Some(vec![build_id]),
                },
            )
            .await?;
        Ok(())
    }
    pub async fn build(
        client: Arc<dagger_sdk::Query>,
        args: &GlobalArgs,
        bin_name: &String,
        _image: &String,
        _tag: &String,
    ) -> eyre::Result<()> {
        // let containers = vec!["linux/amd64", "linux/arm64"];

        let base_image = get_base_debian_image(
            client.clone(),
            &args.clone(),
            Some("linux/amd64".to_string()),
        )
        .await?;

        let container = base_golang_image(
            client.clone(),
            args,
            &Some("linux/amd64".to_string()),
            &bin_name.clone(),
        )
        .await?;
        let build_image = execute(client.clone(), args, &container, &base_image, bin_name).await?;

        build_image.exit_code().await?;

        Ok(())
    }
    pub async fn execute(
        _client: Arc<dagger_sdk::Query>,
        _args: &GlobalArgs,
        container: &dagger_sdk::Container,
        base_image: &dagger_sdk::Container,
        bin_name: &String,
    ) -> eyre::Result<Container> {
        let final_image = base_image
            .with_file(
                format!("/usr/local/bin/{}", &bin_name),
                container
                    .file(format!("/mnt/src/dist/{}", &bin_name))
                    .id()
                    .await?,
            )
            .with_exec(vec![bin_name, "--help"]);

        let output = final_image.stdout().await?;
        println!("{output}");

        Ok(final_image)
    }
}

mod test {
    use std::sync::Arc;

    use crate::GlobalArgs;

    pub async fn execute(
        _client: Arc<dagger_sdk::Query>,
        args: &GlobalArgs,
        container: dagger_sdk::Container,
    ) -> eyre::Result<()> {
        let test_image = container
            .pipeline("test")
            .with_exec(vec!["go", "test", "./..."]);

        test_image.exit_code().await?;

        Ok(())
    }
}

pub async fn get_base_docker_image(
    client: Arc<dagger_sdk::Query>,
    args: &GlobalArgs,
    platform: Option<String>,
) -> eyre::Result<dagger_sdk::Container> {
    let default_platform = client.default_platform().await?;
    let platform = platform.map(Platform).unwrap_or(default_platform);

    let image = client
        .container_opts(QueryContainerOpts {
            id: None,
            platform: Some(platform),
        })
        .from(
            args.docker_image
                .clone()
                .unwrap_or("docker:dind".to_string()),
        );

    Ok(image)
}

pub async fn get_base_debian_image(
    client: Arc<dagger_sdk::Query>,
    args: &GlobalArgs,
    platform: Option<String>,
) -> eyre::Result<dagger_sdk::Container> {
    let docker_image = get_base_docker_image(client.clone(), args, platform.clone()).await?;

    let default_platform = client.default_platform().await?;
    let platform = platform.map(Platform).unwrap_or(default_platform);

    let image = client
        .container_opts(QueryContainerOpts {
            id: None,
            platform: Some(platform),
        })
        .from(
            args.production_image
                .clone()
                .unwrap_or("alpine:latest".to_string()),
        );

    let base_image = image
        .with_exec(vec!["apk", "add", "openssl", "openssl-dev", "pkgconfig"])
        .with_file(
            "/usr/local/bin/docker",
            docker_image.file("/usr/local/bin/docker").id().await?,
        );

    Ok(base_image)
}

pub fn get_src(
    client: Arc<dagger_sdk::Query>,
    args: &GlobalArgs,
) -> eyre::Result<dagger_sdk::Directory> {
    let directory = client.host().directory_opts(
        args.source
            .clone()
            .unwrap_or(PathBuf::from("."))
            .display()
            .to_string(),
        dagger_sdk::HostDirectoryOptsBuilder::default()
            .exclude(vec![
                "node_modules/",
                ".git/",
                "target/",
                ".cuddle/",
                "docs/",
                "ci/",
            ])
            .build()?,
    );

    Ok(directory)
}

pub async fn get_golang_dep_src(
    client: Arc<dagger_sdk::Query>,
    args: &GlobalArgs,
) -> eyre::Result<dagger_sdk::Directory> {
    let directory = client.host().directory_opts(
        args.source
            .clone()
            .unwrap_or(PathBuf::from("."))
            .display()
            .to_string(),
        dagger_sdk::HostDirectoryOptsBuilder::default()
            .include(vec!["**/go.*"])
            .build()?,
    );

    Ok(directory)
}

pub async fn base_golang_image(
    client: Arc<dagger_sdk::Query>,
    args: &GlobalArgs,
    platform: &Option<String>,
    bin_name: &String,
) -> eyre::Result<dagger_sdk::Container> {
    let dep_src = get_golang_dep_src(client.clone(), args).await?;
    let src = get_src(client.clone(), args)?;

    let client = client.pipeline("golang_base_image");

    let goarch = match platform
        .clone()
        .unwrap_or("linux/amd64".to_string())
        .as_str()
    {
        "linux/amd64" => "amd64",
        "linux/arm64" => "arm64",
        _ => eyre::bail!("architecture not supported"),
    };
    let goos = match platform
        .clone()
        .unwrap_or("linux/amd64".to_string())
        .as_str()
    {
        "linux/amd64" => "linux",
        "linux/arm64" => "linux",
        _ => eyre::bail!("os not supported"),
    };
    let golang_build_image = client
        .container()
        .from(
            args.golang_builder_image
                .as_ref()
                .unwrap_or(&"golang:latest".into()),
        )
        .with_env_variable("GOOS", goos)
        .with_env_variable("GOARCH", goarch)
        .with_env_variable("CGO_ENABLED", "0");

    let golang_dep_download = golang_build_image
        .with_directory("/mnt/src", dep_src.id().await?)
        .with_exec(vec!["go", "mod", "download"])
        .with_mounted_cache(
            "/root/go",
            client.cache_volume("golang_mod_cache").id().await?,
        );

    let golang_bin = golang_build_image
        .with_workdir("/mnt/src")
        // .with_directory(
        //     "/root/go",
        //     golang_dep_download.directory("/root/go").id().await?,
        // )
        .with_directory("/mnt/src/", src.id().await?)
        .with_exec(vec![
            "go",
            "build",
            "-o",
            &format!("dist/{bin_name}"),
            "main.go",
        ]);

    golang_bin.exit_code().await?;

    Ok(golang_bin)
}
