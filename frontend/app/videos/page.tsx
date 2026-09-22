'use client';
import { Center, Container, Title } from "@mantine/core";
import GanymedeLoadingText from "../components/utils/GanymedeLoadingText";
import { useFetchVideosFilter, VideoSortBy } from "../hooks/useVideos";
import { useVideoListParams } from "../hooks/useVideoListParams";
import useSettingsStore from "../store/useSettingsStore";
import VideoGrid from "../components/videos/Grid";
import ChannelRows from "../components/videos/ChannelRows";
import { useTranslations } from "next-intl";
import { usePageTitle } from "../util/util";

// The Videos page also offers the UI-only channel grouping
const SORT_OPTIONS = Object.values(VideoSortBy);

const VideosPage = () => {
  const t = useTranslations("VideosPage");
  usePageTitle(t('title'));

  const { page, videoTypes, sortBy, order, setPage, setVideoTypes, setSortBy, setOrder } = useVideoListParams(SORT_OPTIONS);
  const byChannel = sortBy === VideoSortBy.Channel;

  const videoLimit = useSettingsStore((state) => state.videoLimit);
  const setVideoLimit = useSettingsStore((state) => state.setVideoLimit);

  // In channel mode the rows fetch their own videos; this request only feeds the total count
  const { data: videos, isPending, isError } = useFetchVideosFilter({
    limit: byChannel ? 1 : videoLimit,
    offset: byChannel ? 0 : (page - 1) * videoLimit,
    types: videoTypes,
    playlist_id: "",
    sort_by: byChannel ? VideoSortBy.Date : sortBy,
    order: order,
  });

  if (isPending) {
    return <GanymedeLoadingText message={t('loading')} />;
  }

  if (isError) {
    return <div>{t('error')}</div>;
  }

  return (
    <div>
      <Center mt={10}>
        <Title>{t('title')}</Title>
      </Center>

      <Container size="xl" px="xl" fluid={true}>
        <VideoGrid
          videos={videos.data}
          totalCount={videos.total_count}
          totalPages={videos.pages}
          currentPage={page}
          onPageChange={setPage}
          isPending={isPending}
          videoLimit={videoLimit}
          onVideoLimitChange={setVideoLimit}
          videoTypes={videoTypes}
          onVideoTypeChange={setVideoTypes}
          sortBy={sortBy}
          onSortByChange={setSortBy}
          sortOptions={SORT_OPTIONS}
          order={order}
          onOrderChange={setOrder}
          showChannel={true}
        >
          {byChannel ? <ChannelRows videoTypes={videoTypes} order={order} /> : undefined}
        </VideoGrid>
      </Container>
    </div>
  );
}

export default VideosPage;
