"use client"
import { useGetVideoByExternalId, Video } from "@/app/hooks/useVideos";
import { durationToTime, escapeURL, formatBytes } from "@/app/util/util";
import { Avatar, Box, Divider, Tooltip, Text, Group, Badge, Button, rem } from "@mantine/core";
import { env } from "next-runtime-env";
import classes from "./TitleBar.module.css";
import { IconCalendarEvent, IconDatabase, IconHourglass, IconLock, IconUser, IconUsers } from "@tabler/icons-react";
import { MediaPlayerInstance } from "@vidstack/react";
import { RefObject, useEffect, useState } from "react";
import dayjs from "dayjs";
import VideoMenu from "./Menu";
import useAuthStore from "@/app/store/useAuthStore";
import { UserRole } from "@/app/hooks/useAuthentication";
import Link from "next/link";
import { useTranslations } from "next-intl";

interface Params {
  video: Video;
  playerRef: RefObject<MediaPlayerInstance | null>;
}

// Time left to watch at the current playback speed.
const VideoRemainingTime = ({ playerRef }: { playerRef: RefObject<MediaPlayerInstance | null> }) => {
  const t = useTranslations("VideoComponents");
  const [remaining, setRemaining] = useState<{ seconds: number; rate: number }>();

  useEffect(() => {
    // Re-render only when the shown second or the speed changes, not on every player tick
    return playerRef.current?.subscribe(({ currentTime, duration, playbackRate }) => {
      if (!duration) return;
      const seconds = Math.ceil(Math.max(0, duration - currentTime) / playbackRate);
      setRemaining((prev) => (prev?.seconds === seconds && prev.rate === playbackRate ? prev : { seconds, rate: playbackRate }));
    });
  }, [playerRef]);

  if (!remaining) return null;

  return (
    <Group mr={15}>
      <Tooltip label={t('remainingTimeTooltip', { rate: remaining.rate })} openDelay={250}>
        <div className={classes.titleBarBadge}>
          <Text mr={3}>{durationToTime(remaining.seconds)}</Text>
          <IconHourglass size={20} />
        </div>
      </Tooltip>
    </Group>
  );
};

const VideoTitleBar = ({ video, playerRef }: Params) => {
  const t = useTranslations("VideoComponents");
  const hasPermission = useAuthStore(state => state.hasPermission);

  const { data: clipFullVideo } = useGetVideoByExternalId(video.clip_ext_vod_id)

  return (
    <div className={classes.titleBarContainer}>
      <div className={classes.titleBar}>
        <Avatar
          component={Link}
          href={`/channels/${video.edges.channel.name}`}
          src={`${(env('NEXT_PUBLIC_CDN_URL') ?? '')}${escapeURL(video.edges.channel.image_path)}`}
          radius="xl"
          alt={video.edges.channel.display_name}
          mr={10}
        />

        <Divider size="sm" orientation="vertical" mr={10} />

        <div style={{ width: "60%" }}>
          <Tooltip label={video.title} openDelay={250}>
            <Text size="xl" lineClamp={1} pt={3}>
              {video.title}
            </Text>
          </Tooltip>
        </div>

        <div className={classes.titleBarRight}>

          <div className={classes.titleBarBadge}>

            {clipFullVideo && (
              <Group mr={15}>
                <Button variant="default" size="xs" component={Link} href={`/videos/${clipFullVideo.id}?t=${video.clip_vod_offset}`}>Go To Full Video</Button>
              </Group>
            )}

            <VideoRemainingTime playerRef={playerRef} />

            {video.views && (
              <Group mr={15}>
                <Tooltip
                  label={`${video.views.toLocaleString()} ${t('sourceViewsTooltip')}`}
                  openDelay={250}
                >
                  <div className={classes.titleBarBadge}>
                    <Text mr={3}>{video.views.toLocaleString()}</Text>
                    <IconUsers size={20} />
                  </div>
                </Tooltip>
              </Group>
            )}

            {video.local_views && (
              <Group mr={15}>
                <Tooltip
                  label={`${video.local_views.toLocaleString()} ${t('localViewsTooltip')}`}
                  openDelay={250}
                >
                  <div className={classes.titleBarBadge}>
                    <Text mr={3}>{video.local_views.toLocaleString()}</Text>
                    <IconUser size={20} />
                  </div>
                </Tooltip>
              </Group>
            )}

            <Group mr={15}>
              <Tooltip
                label={`${t('streamedOnTooltip')} ${new Date(
                  video.streamed_at
                ).toLocaleString()}`}
                openDelay={250}
              >
                <div className={classes.titleBarBadge}>
                  <Text mr={5}>
                    {dayjs(video.streamed_at).format("YYYY/MM/DD")}
                  </Text>
                  <IconCalendarEvent size={20} />
                </div>
              </Tooltip>
            </Group>

            <Group mr={15}>
              <Tooltip
                label={`${t('storageSizeTooltip')}`}
                openDelay={250}
              >
                <div className={classes.titleBarBadge}>
                  <Text mr={5}>
                    {formatBytes(video.storage_size_bytes ?? 0, 0)}
                  </Text>
                  <IconDatabase size={20} />
                </div>
              </Tooltip>
            </Group>

            {video.locked && (
              <Group mr={5}>
                <Tooltip label={t('lockedText')} openDelay={250}>
                  <div className={classes.titleBarBadge}>
                    <Badge variant="default" leftSection={<IconLock style={{ width: rem(12), height: rem(12) }} />}>
                      {t('locked')}
                    </Badge>
                  </div>
                </Tooltip>
              </Group>
            )}

            <Group>
              <Tooltip label={t('videoTypeTooltip')} openDelay={250}>
                {video.processing ? (
                  <div className={classes.titleBarBadge}>
                    <Badge color="red">
                      {video.type} - {t('processingOverlayText')}
                    </Badge>
                  </div>
                ) : (
                  <div className={classes.titleBarBadge}>
                    <Badge variant="default">
                      {video.type}
                    </Badge>
                  </div>
                )}

              </Tooltip>
            </Group>
          </div>

          {hasPermission(UserRole.Archiver) && (
            <Box mt={5}>
              <VideoMenu video={video} />
            </Box>
          )}

        </div>
      </div>
    </div >
  );
};

export default VideoTitleBar;