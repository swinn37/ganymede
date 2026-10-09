"use client"
import { Box, Center, Container, Title, useMantineTheme } from "@mantine/core";
import { useElementSize, useMediaQuery, useViewportSize } from "@mantine/hooks";
import useAuthStore from "./store/useAuthStore";
import { LandingHero } from "./components/landing/Hero";
import ContinueWatching from "./components/landing/ContinueWatching";
import RecentlyArchived from "./components/landing/RecentlyArchived";
import { useEffect } from "react";
import { useTranslations } from "next-intl";

// The grids take as many columns of cards at least CARD_MIN_WIDTH wide as fit, and the recent archives
// as many rows as fit in the window. Heights are estimates of the page layout.
const CARD_MIN_WIDTH = 300;
const GRID_GAP = 10; // SimpleGrid spacing "xs"
const CARD_TEXT_HEIGHT = 110; // title, channel and date under the thumbnail
const SECTION_TITLE_HEIGHT = 60;
const NAVBAR_HEIGHT = 56;
// The former fixed counts: the mobile carousels keep them and the grids never show fewer videos
const CONTINUE_WATCHING_COUNT = 4;
const RECENTLY_ARCHIVED_COUNT = 8;

export default function Home() {
  const { isLoggedIn, hasHydrated } = useAuthStore();
  const theme = useMantineTheme();
  const isMobile = useMediaQuery(`(max-width: ${theme.breakpoints.sm})`);
  const { ref, width } = useElementSize();
  const { height } = useViewportSize();

  useEffect(() => {
    document.title = "Ganymede";
  }, []);

  const t = useTranslations("HomePage");

  const columns = Math.max(1, Math.floor((width + GRID_GAP) / (CARD_MIN_WIDTH + GRID_GAP)));
  const rowHeight = ((width - GRID_GAP * (columns - 1)) / columns) * 9 / 16 + CARD_TEXT_HEIGHT + GRID_GAP;
  const continueRows = Math.ceil(CONTINUE_WATCHING_COUNT / columns);
  const continueWatchingHeight = isLoggedIn ? SECTION_TITLE_HEIGHT + continueRows * rowHeight : 0;
  const recentRows = Math.max(Math.ceil(RECENTLY_ARCHIVED_COUNT / columns),
    Math.floor((height - NAVBAR_HEIGHT - continueWatchingHeight - SECTION_TITLE_HEIGHT) / rowHeight));

  return (
    <div>
      {/* Render neither variant until the auth state is known, otherwise the hero flashes for logged-in users */}
      {hasHydrated && !isLoggedIn && (
        <Box mb={5}>
          <LandingHero />
        </Box>
      )}

      <Container fluid ref={ref}>
        {hasHydrated && isLoggedIn && (
          <Box>
            <Center>
              <Title>{t('continueWatching')}</Title>
            </Center>
            {/* Wait for the measured width so the first request already asks for the right count */}
            {width > 0 && (
              <Box mt={10}>
                <ContinueWatching columns={columns} count={isMobile ? CONTINUE_WATCHING_COUNT : columns * continueRows} />
              </Box>
            )}
          </Box>
        )}

        <Box>
          <Center>
            <Title>{t('recentlyArchived')}</Title>
          </Center>
          {width > 0 && (
            <Box mt={10}>
              <RecentlyArchived columns={columns} count={isMobile ? RECENTLY_ARCHIVED_COUNT : columns * recentRows} />
            </Box>
          )}
        </Box>
      </Container>
    </div>
  );
}
