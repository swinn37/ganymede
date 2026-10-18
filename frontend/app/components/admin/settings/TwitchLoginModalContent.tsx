import {
  ActionIcon,
  Alert,
  Button,
  Code,
  CopyButton,
  Group,
  Loader,
  Stack,
  Text,
  ThemeIcon,
  Tooltip,
} from "@mantine/core";
import { IconAlertTriangle, IconBrandTwitch, IconCheck, IconCopy } from "@tabler/icons-react";
import { useTranslations } from "next-intl";
import { useEffect, useRef, useState } from "react";
import { useAxiosPrivate } from "@/app/hooks/useAxios";
import { TwitchLogin, usePollTwitchLogin, useStartTwitchLogin } from "@/app/hooks/useConfig";

type Props = {
  // Called with the token once the admin authorized it on Twitch; the backend has already saved it.
  onAuthorized: (token: string) => void;
  onClose: () => void;
};

const POPUP_WIDTH = 520;
const POPUP_HEIGHT = 760;

const TwitchLoginModalContent = ({ onAuthorized, onClose }: Props) => {
  const t = useTranslations("AdminSettingsPage.videoSettings.twitchLogin");
  const axiosPrivate = useAxiosPrivate();
  const startMutation = useStartTwitchLogin();
  const pollMutation = usePollTwitchLogin();
  const [login, setLogin] = useState<TwitchLogin>();
  const [failed, setFailed] = useState(false);
  // Set once authorized: the linked account, or "" when Twitch did not report it.
  const [account, setAccount] = useState<string>();
  const popup = useRef<Window | null>(null);

  // The Twitch window is useless once the login ends, whatever the outcome.
  const closePopup = () => {
    popup.current?.close();
    popup.current = null;
  };

  const openPopup = () => {
    if (!login) return;
    const left = window.screenX + Math.max(0, (window.outerWidth - POPUP_WIDTH) / 2);
    const top = window.screenY + Math.max(0, (window.outerHeight - POPUP_HEIGHT) / 2);
    popup.current = window.open(
      login.verification_uri,
      "ganymede-twitch-login",
      `popup=yes,width=${POPUP_WIDTH},height=${POPUP_HEIGHT},left=${left},top=${top}`,
    );
  };

  const start = () => {
    closePopup();
    setFailed(false);
    setLogin(undefined);
    startMutation.mutate(axiosPrivate, {
      onSuccess: setLogin,
      onError: () => setFailed(true),
    });
  };

  useEffect(() => {
    start();
    return closePopup;
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  // Poll until the code is authorized, expires, or the modal closes.
  useEffect(() => {
    if (!login) return;
    let active = true;
    let timer: ReturnType<typeof setTimeout>;
    const deadline = Date.now() + login.expires_in * 1000;

    const fail = () => {
      closePopup();
      setFailed(true);
    };

    const poll = async () => {
      if (Date.now() > deadline) {
        fail();
        return;
      }
      try {
        const result = await pollMutation.mutateAsync({ axiosPrivate, deviceCode: login.device_code });
        if (!active) return;
        if (result.status === "authorized" && result.twitch_token) {
          closePopup();
          setAccount(result.twitch_login ?? "");
          onAuthorized(result.twitch_token);
          return;
        }
        timer = setTimeout(poll, login.interval * 1000);
      } catch {
        if (active) fail();
      }
    };
    timer = setTimeout(poll, login.interval * 1000);

    return () => {
      active = false;
      clearTimeout(timer);
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [login]);

  if (account !== undefined) {
    return (
      <Stack gap="md" align="center">
        <ThemeIcon color="teal" size={56} radius="xl">
          <IconCheck size={32} />
        </ThemeIcon>
        <Text fw={700} size="lg">
          {t("successTitle")}
        </Text>
        {account && <Text>{t.rich("successAccount", { account, b: (chunks) => <b>{chunks}</b> })}</Text>}
        <Text size="sm" c="dimmed" ta="center">
          {t("successMessage")}
        </Text>
        <Button fullWidth onClick={onClose}>
          {t("close")}
        </Button>
      </Stack>
    );
  }

  if (failed) {
    return (
      <Stack gap="md">
        <Alert color="red" icon={<IconAlertTriangle size={18} />}>
          {t("failed")}
        </Alert>
        <Button onClick={start}>{t("retry")}</Button>
      </Stack>
    );
  }

  if (!login) {
    return (
      <Group justify="center" py="md">
        <Loader />
      </Group>
    );
  }

  return (
    <Stack gap="md">
      <Text size="sm">{t("intro")}</Text>
      <Button color="violet" leftSection={<IconBrandTwitch size={18} />} onClick={openPopup}>
        {t("open")}
      </Button>

      <Text size="sm">{t("step2")}</Text>
      <Group gap="xs" justify="center" wrap="nowrap">
        <Code fz="xl" fw={700} style={{ letterSpacing: "0.2em" }}>
          {login.user_code}
        </Code>
        <CopyButton value={login.user_code} timeout={2000}>
          {({ copied, copy }) => (
            <Tooltip label={copied ? t("copied") : t("copy")}>
              <ActionIcon
                size="lg"
                variant="light"
                color={copied ? "teal" : "blue"}
                onClick={copy}
                aria-label={t("copy")}
              >
                {copied ? <IconCheck size={18} /> : <IconCopy size={18} />}
              </ActionIcon>
            </Tooltip>
          )}
        </CopyButton>
      </Group>

      <Group gap="xs" wrap="nowrap">
        <Loader size="xs" />
        <Text size="sm" c="dimmed">
          {t("waiting")}
        </Text>
      </Group>
    </Stack>
  );
};

export default TwitchLoginModalContent;
