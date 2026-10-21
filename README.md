<br />
<div align="center">
  <a>
    <img src=".github/ganymede-logo.png" alt="Logo" width="80" height="80">
  </a>

  <h2 align="center">Ganymede</h2>

  <p align="center">
    Ganymede is a Twitch VOD and Live Stream archiving platform with a real-time chat experience. Every archive includes a rendered chat for viewing outside of Ganymede. Files are saved in a friendly format allowing for use without Ganymede.
  </p>
</div>

---

## Fork changes

This is a fork of [Zibbp/ganymede](https://github.com/Zibbp/ganymede), based on v5.0.0. It adds the changes below; everything else follows upstream. The image is published as `ghcr.io/swinn37/ganymede:fork-v5`.

### Live archives resumed after a stream drop, in parts

Two settings in **Admin › Settings › Video**, under *Live Stream*, both `0` (upstream behaviour) by default:

- **Live Stream Reconnect Window (minutes)** (`livestream.reconnect_grace_minutes`): when a stream drops and comes back within that many minutes, keeps archiving it into the same video instead of starting a new one. The live chat is lined up with the video, without the time the stream was down.
- **Live Archive Part Length (minutes)** (`livestream.split_duration_minutes`): saves live archives as HLS in MP4 parts of at most that many minutes, played in Ganymede as a single video. Each part plays on its own (VLC, mpv, Jellyfin), and a resumed stream also starts a new part:

```
{channel}/{folder}/{file}-video_hls/
  {streamId}-video.m3u8         # video_path, played as one video
  {streamId}-video-part001.mp4
  {streamId}-video-part002.mp4
```

A resumed capture records each run to its own playlist and init segment, and the capture playlist stitches the runs together, so the recording can still be watched while it continues. A part keeps the timeline of the whole video: a later part starts at its time in the video.

Limitations: without parts or *Save as HLS*, the runs are exported into a single MP4, which assumes the stream comes back with the same encoding settings. The absolute time display and multistream sync do not account for the time a stream was down.

### Connect with Twitch

- **Admin › Settings › Video** has a **Connect with Twitch** button next to the Twitch token. It opens a small Twitch window with the code already filled in. After you click **Authorize**, Ganymede saves the token, closes the Twitch window and shows the linked account. Pasting a token by hand still works.
- The login goes through Twitch's TV app, the only Twitch client that still offers this flow with a token valid for subscriber-only VODs and ad-free live streams. Twitch shows the request as coming from *Twitch for TV*.
- Requests that send the token now use the TV app's Client-ID, like yt-dlp, so the token and Client-ID match. Tokens copied from the website work with it too.
- The token does not expire. It stops working if you remove the Twitch TV app from your Twitch connections or change your password.

### Player

- A speed button next to the settings shows the current speed and offers 0.25x to 2.5x. *Settings › Speed* and the `>` / `<` shortcuts also go up to 2.5x (Video.js stops at 2x).
- The chosen speed is remembered in the browser and applied to the next videos. The LIVE button's switch to 1x does not change it.
- The volume popup shows the volume as a percentage. The mouse wheel changes the volume in 5% steps over the volume button, and the speed in 0.1x steps over the speed button, without scrolling the page.
- The controls hide 1.2 s after the mouse stops moving (Video.js waits 2 s), and stay visible while the mouse rests on them.
- The bar under the player shows the time left to watch at the current speed, and the channel avatar opens the channel page.

### Watching while archiving

- A recording in progress plays like a regular video: from the start, with a timeline that grows as the recording continues, speed control and shortcuts. As on Twitch, a red **LIVE** badge at the end of the timeline, before the time left, shows you are watching the latest part. When you are behind, it becomes a **LIVE ▶|** button that jumps back to it at 1x speed.
- Once its video is in place, an archive plays while its chat is still processed, instead of showing that no preview is available.

### Home page

- The *Continue Watching* and *Recently Archived* grids use the full width of the window, with as many columns of cards at least 300 px wide as fit. *Continue Watching* shows one full row, and *Recently Archived* as many full rows as fit in the window. They never show fewer videos than the former fixed grids (4 and 8), and the mobile carousels are unchanged.

### Videos page

- New **Channel** option in *Sort by*: one horizontally scrolling row per channel with its 12 latest videos (the type filter and order apply) and a link to the channel page. Rows load as they scroll into view.

### French translation

- The interface is fully translated into French: pick **Français** in the language menu, or set `DEFAULT_LOCALE=fr` to use it for everyone.

### Docker image

- Runs on Debian 13 (trixie) instead of Debian 12, with the security updates published since the base image applied at build time.
- `gosu`, an outdated Go build flagged by scanners, is replaced by `setpriv` from util-linux. The frontend ships only the `node` binary, without npm and its dependencies, and the unused `python3-pip` is gone.
- yt-dlp 2026.08.19, and golang.org/x/net 0.60.0 in the API and worker.
- Trivy 0.74.0 finds 359 vulnerabilities in this image, none critical and none with an available fix, against 593 in the upstream 5.0.0 image (6 critical, 103 with a fix).

---

## Screenshot

![ganymede-readme_landing](https://github.com/user-attachments/assets/b1c024f5-f5ad-4611-84db-42d599364a74)

https://github.com/user-attachments/assets/184451f1-e3ce-4329-8516-a9842648c01b

## About

Ganymede allows archiving of past streams (VODs) and live streams with a real-time chat playback along with a archival-friendly rendered chat. All files are saved in a friendly way that doesn't require Ganymede to view them (see [file structure](https://github.com/Zibbp/ganymede/wiki/File-Structure)). Ganymede is the successor of [Ceres](https://github.com/Zibbp/Ceres).

## Features

- Realtime Chat Playback
- SSO / OAuth authentication ([wiki](https://github.com/Zibbp/ganymede/wiki/SSO---OpenID-Connect))
- Light/dark mode toggle.
- 'Watched channels'
  - Allows watching channels for archiving past broadcasts and live streams. Includes advanced filtering options.
- Twitch VOD/Livestream support.
- Full VOD, Channel, and User management.
- Custom post-download video FFmpeg parameters.
- Custom chat render parameters.
- Webhook notifications.
- Simple file structure for long-term archival that will outlast Ganymede.
- Recoverable queue system.
- Playback / progress saving.
- Playlists.

## Documentation

For in-depth documentation on features visit the [wiki](https://github.com/Zibbp/ganymede/wiki).

## API

Visit the [docs](https://github.com/Zibbp/ganymede/tree/main/docs) folder for the API docs.

## Translations

See the [messages](https://github.com/Zibbp/ganymede/tree/main/frontend/messages) directory for available translations. If you would like to add a new translation, please create a pull request with the new translation file. The file should be named `<language>.json` where `<language>` is the language code (e.g. `de.json` for German). Additionally the language needs to be added to the navbar in the `frontend/app/layout/Navbar.tsx` file in the `languages` array. Use the `frontend/translation-coverage.js` script to see what has been translated or with the `-u` option to populate missing keys in translation files. 

## Installation

### Requirements

- Linux environment with Docker.
- _Optional_ network mounted storage.
- 50gb+ free storage, see [storage requirements](https://github.com/Zibbp/ganymede/wiki/Storage-Requirements).
- A Twitch Application
  - [Create an application](https://dev.twitch.tv/console/apps/create).

### Installation

Ganymede consists of two docker containers:

1. Server
2. Postgres Database

Feel free to use an existing Postgres database container if you don't want to spin new ones up.

1. Download a copy of the `docker-compose.yml` file.
2. Edit the `docker-compose.yml` file modifying the environment variables, see [environment variables](https://github.com/Zibbp/ganymede#environment-variables) for more information.
3. Run `docker compose up -d`.
4. Visit the address and port you specified for the frontend and login with username: `admin` password: `ganymede`.
5. Change the admin password _or_ create a new user, grant admin permissions on that user, and delete the admin user.

### Images

Images are published to ghcr.io. There are four types of tags available.

| Tag                  | Description                                                              |
| -------------------- | ------------------------------------------------------------------------ |
| latest               | Points to the latest release.                                            |
| semver - e.g. 4.10.0 | The latest release in semver format.                                     |
| dev                  | Build of the 'main' branch. Use for testing new features before release. |
| pr-* - e.g. pr-123   | Build of a pull request. Gets deleted when the PR is closed.             |

### Rootless

The API container can be run as a non root user. To do so add `PUID` and `PGID` environment variables, setting the value to your user. Read [linuxserver's docs](https://docs.linuxserver.io/general/understanding-puid-and-pgid) about this for more information.

Note: On startup the container will `chown` the config, temp, and logs directory. It will not recursively `chown` the `/data/videos` directory. Ensure the mounted `/data/videos` directory is readable by the set user.

If your storage does not support `chown`, such as an NFS or SMB mount (e.g. TrueNAS with NFSv4 ACLs), set `SKIP_CHOWN=true` to skip this step. Ensure the directories are already writable by the set user.

### Config

A configuration file is generate on initial start of Ganymede. By default the configuration is at `/data/config/config.json`. See the [config.go](https://github.com/Zibbp/ganymede/blob/main/internal/config/config.go) file for a full list of configuration settings. Most of the settings can be configured in the Web UI by navigating to Admin > Settings.

### Environment Variables

The `docker-compose.yml` file has comments for each environment variable. Below is a list of all environment variables and their descriptions. See the [env.go](https://github.com/Zibbp/ganymede/blob/main/internal/config/env.go) file for a full list of all environment variables and their default values.

##### Server

| ENV Name                                | Description                                                                                                                     |
| --------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------- |
| `DEBUG`                                 | Enable debug logging `true` or `false`.                                                                                         |
| `VIDEOS_DIR`                            | Path inside the container to the videos directory. Default: `/data/videos`.                                                     |
| `TEMP_DIR`                              | Path inside the container where temporary files are stored during archiving. Default: `/data/temp`.                             |
| `LOGS_DIR`                              | Path inside the container where log files are stored. Default: `/data/logs`.                                                    |
| `CONFIG_DIR`                            | Path inside the container where the config is stored. Default: `/data/config`.                                                  |
| `SKIP_CHOWN`                            | _Optional_ Skip `chown` of the data directories on startup. Useful when storage does not support `chown`, e.g. NFS/SMB. Default: `false`. |
| `PATH_MIGRATION_ENABLED`                | Enable path migration at startup. Default: `true`.                                                                              |
| `TZ`                                    | Timezone.                                                                                                                       |
| `DB_HOST`                               | Host of the database.                                                                                                           |
| `DB_PORT`                               | Port of the database.                                                                                                           |
| `DB_USER`                               | Username for the database.                                                                                                      |
| `DB_PASS`                               | Password for the database.                                                                                                      |
| `DB_NAME`                               | Name of the database.                                                                                                           |
| `DB_SSL`                                | Whether to use SSL. Default: `disable`. See [DB SSL](https://github.com/Zibbp/ganymede/wiki/DB-SSL) for more information.       |
| `DB_SSL_ROOT_CERT`                      | _Optional_ Path to DB SSL root certificate. See [DB SSL](https://github.com/Zibbp/ganymede/wiki/DB-SSL) for more information.   |
| `TWITCH_CLIENT_ID`                      | Twitch application client ID.                                                                                                   |
| `TWITCH_CLIENT_SECRET`                  | Twitch application client secret.                                                                                               |
| `OAUTH_ENABLED`                         | _Optional_ Wether OAuth is enabled `true` or `false`. Must have the other OAuth variables set if this is enabled.               |
| `OAUTH_PROVIDER_URL`                    | _Optional_ OAuth provider URL. See https://github.com/Zibbp/ganymede/wiki/SSO---OpenID-Connect                                  |
| `OAUTH_CLIENT_ID`                       | _Optional_ OAuth client ID.                                                                                                     |
| `OAUTH_CLIENT_SECRET`                   | _Optional_ OAuth client secret.                                                                                                 |
| `OAUTH_REDIRECT_URL`                    | _Optional_ OAuth redirect URL, points to the API. Example: `http://localhost:4000/api/v1/auth/oauth/callback`.                  |
| `MAX_CHAT_DOWNLOAD_EXECUTIONS`          | Maximum number of chat downloads that can be running at once. Live streams bypass this limit.                                   |
| `MAX_CHAT_RENDER_EXECUTIONS`            | Maximum number of chat renders that can be running at once.                                                                     |
| `MAX_VIDEO_DOWNLOAD_EXECUTIONS`         | Maximum number of video downloads that can be running at once. Live streams bypass this limit.                                  |
| `MAX_VIDEO_CONVERT_EXECUTIONS`          | Maximum number of video conversions that can be running at once.                                                                |
| `MAX_VIDEO_SPRITE_THUMBNAIL_EXECUTIONS` | Maximum number of video sprite thumbnail generation jobs that can be running at once. This is not very CPU intensive.           |
| `DISABLE_LIVE_ARCHIVE_STALL_CHECK`       | Disable FFmpeg progress based stall detection for live archives. Default: `false`.                                              |
| `LIVE_ARCHIVE_STALL_TIMEOUT_SECONDS`    | Seconds without FFmpeg media progress before a live capture is stopped. Default: `180`; values below `30` use the default.     |
| `SHOW_SSO_LOGIN_BUTTON`                 | Frontend: `true/false` Show a "login via sso" button on the login page (defaults to false).                                     |
| `FORCE_SSO_AUTH`                        | Frontend: `true/false` Force users to login via SSO by bypassing the login page (defaults to false).                            |
| `REQUIRE_LOGIN`                         | Frontend: `true/false` Require users to be logged in to view videos (defaults to false).                                        |
| `SHOW_LOCALE_BUTTON`                    | Frontend: `true/false` Show the locale/language button on the navbar (defaults to true).                                        |
| `DEFAULT_LOCALE`                        | Frontend: Sets the default locale/language. Must be the short code of the language. Example: `en` for English, `de` for German. |
| `FORCE_LOGIN`                           | Frontend: `true/false` Force require users to login to view any page (defaults to false).                                       |

##### DB

**Ensure these are the same in the API environment variables.**

| ENV Name            | Description           |
| ------------------- | --------------------- |
| `POSTGRES_PASSWORD` | Database password     |
| `POSTGRES_USER`     | Database username.    |
| `POSTGRES_DB`       | Name of the database. |

### Volumes

##### API

| Volume         | Description                                                                                                                                                                                       | Example                      |
| -------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ---------------------------- |
| `/data/videos` | Mount for video storage. This **must** match the `VIDEOS_DIR` environment variable.                                                                                                               | `/mnt/nas/vods:/data/videos` |
| `/data/logs`   | Mount to store task logs. This **must** match the `LOGS_DIR` environment variable.                                                                                                                | `./logs:/data/logs`          |
| `/data/temp`   | Mount to store temporary files during the archive process. This is mounted to the host so files are recoverable in the event of a crash. This **must** match the `TEMP_DIR` environment variable. | `./temp:/data/temp`          |
| `/data/config` | Mount to store the config. This **must** match the `CONFIG_DIR` environment variable.                                                                                                             | `./config:/data/config`      |


## Development

A [devcontainer](https://containers.dev/) is included for development. This container includes all the necessary tools to develop Ganymede. Once setup, the [Makefile](/Makefile) can be used to run the development environment.

- `make dev_server` - Starts the server.
- `make dev_worker` - Starts the worker.
- `make dev_web` - Starts the web server.

View the [Makefile](/Makefile) for more commands.

## Acknowledgements

- [TwitchDownloader](https://github.com/lay295/TwitchDownloader)
- [yt-dlp](https://github.com/yt-dlp/yt-dlp)

## License

[GNU General Public License v3.0](https://github.com/Zibbp/ganymede/blob/main/LICENSE)
