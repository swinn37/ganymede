import { Anchor, Avatar, Box, Group, Text, Title } from "@mantine/core";
import { Carousel } from "@mantine/carousel";
import { useInViewport } from "@mantine/hooks";
import Link from "next/link";
import { useEffect, useState } from "react";
import { useTranslations } from "next-intl";
import { env } from "next-runtime-env";
import { Channel, useFetchChannels } from "@/app/hooks/useChannels";
import { useFetchVideosFilter, VideoOrder, VideoSortBy, VideoType } from "@/app/hooks/useVideos";
import { escapeURL } from "@/app/util/util";
import VideoCard from "./Card";
import GanymedeLoadingText from "../utils/GanymedeLoadingText";

const ROW_VIDEO_LIMIT = 12;
const ROW_PLACEHOLDER_HEIGHT = 320;

type RowFilters = {
  videoTypes: VideoType[];
  order: VideoOrder;
};

const ChannelRow = ({ channel, videoTypes, order }: RowFilters & { channel: Channel }) => {
  const t = useTranslations("VideoComponents");
  const { data } = useFetchVideosFilter({
    limit: ROW_VIDEO_LIMIT,
    offset: 0,
    channel_id: channel.id,
    types: videoTypes,
    sort_by: VideoSortBy.Date,
    order: order,
  });

  if (!data || data.total_count === 0) return null;

  return (
    <Box mb="lg">
      <Group justify="space-between" mb="xs">
        <Group gap="xs">
          <Avatar
            src={`${(env('NEXT_PUBLIC_CDN_URL') ?? '')}${escapeURL(channel.image_path)}`}
            size={32}
            radius="xl"
          />
          <Title order={3}>{channel.display_name}</Title>
          <Text c="dimmed">{t('channelRowVideosCount', { count: data.total_count })}</Text>
        </Group>
        <Anchor component={Link} href={`/channels/${channel.name}`}>
          {t('channelRowSeeAll')}
        </Anchor>
      </Group>
      <Carousel
        slideSize={{ base: '100%', sm: '50%', md: '33.333333%', lg: '25%', xl: '20%', xxl: '16.666666%' }}
        slideGap="xs"
        emblaOptions={{ align: "start" }}
      >
        {data.data.map((video) => (
          <Carousel.Slide key={video.id}>
            <VideoCard video={video} showChannel={false} showMenu={true} showProgress={true} />
          </Carousel.Slide>
        ))}
      </Carousel>
    </Box>
  );
};

// A row only fetches once it scrolls into view: each card also requests its playback progress.
const LazyChannelRow = (props: RowFilters & { channel: Channel }) => {
  const { ref, inViewport } = useInViewport();
  const [seen, setSeen] = useState(false);

  useEffect(() => {
    if (inViewport) setSeen(true);
  }, [inViewport]);

  return (
    <div ref={ref} style={{ minHeight: seen ? undefined : ROW_PLACEHOLDER_HEIGHT }}>
      {seen && <ChannelRow {...props} />}
    </div>
  );
};

// One horizontal row of recent videos per channel, channels sorted by name.
const ChannelRows = ({ videoTypes, order }: RowFilters) => {
  const t = useTranslations("VideoComponents");
  const { data: channels, isPending, isError } = useFetchChannels();

  if (isPending) return <GanymedeLoadingText message={t('loadingVideos')} />;
  if (isError) return null;

  const sortedChannels = [...channels].sort((a, b) =>
    a.display_name.localeCompare(b.display_name, undefined, { sensitivity: "base" })
  );

  return (
    <>
      {sortedChannels.map((channel) => (
        <LazyChannelRow key={channel.id} channel={channel} videoTypes={videoTypes} order={order} />
      ))}
    </>
  );
};

export default ChannelRows;
