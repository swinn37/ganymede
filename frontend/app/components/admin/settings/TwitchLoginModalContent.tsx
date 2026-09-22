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
  Tooltip,
} from "@mantine/core";
import { IconAlertTriangle, IconCheck, IconCopy, IconExternalLink } from "@tabler/icons-react";
import { useTranslations } from "next-intl";
import { useEffect, useState } from "react";
import { useAxiosPrivate } from "@/app/hooks/useAxios";
import { TwitchLogin, usePollTwitchLogin, useStartTwitchLogin } from "@/app/hooks/useConfig";

type Props = {
  // Called with the token once the admin authorized it on Twitch; the backend has already saved it.
  onAuthorized: (token: string) => void;
};

const TwitchLoginModalContent = ({ onAuthorized }: Props) => {
  const t = useTranslations("AdminSettingsPage.videoSettings.twitchLogin");
  const axiosPrivate = useAxiosPrivate();
  const startMutation = useStartTwitchLogin();
  const pollMutation = usePollTwitchLogin();
  const [login, setLogin] = useState<TwitchLogin>();
  const [failed, setFailed] = useState(false);

  const start = () => {
    setFailed(false);
    setLogin(undefined);
    startMutation.mutate(axiosPrivate, {
      onSuccess: setLogin,
      onError: () => setFailed(true),
    });
  };

  useEffect(() => {
    start();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  // Poll until the code is authorized, expires, or the modal closes.
  useEffect(() => {
    if (!login) return;
    let active = true;
    let timer: ReturnType<typeof setTimeout>;
    const deadline = Date.now() + login.expires_in * 1000;

    const poll = async () => {
      if (Date.now() > deadline) {
        setFailed(true);
        return;
      }
      try {
        const result = await pollMutation.mutateAsync({ axiosPrivate, deviceCode: login.device_code });
        if (!active) return;
        if (result.status === "authorized" && result.twitch_token) {
          onAuthorized(result.twitch_token);
          return;
        }
        timer = setTimeout(poll, login.interval * 1000);
      } catch {
        if (active) setFailed(true);
      }
    };
    timer = setTimeout(poll, login.interval * 1000);

    return () => {
      active = false;
      clearTimeout(timer);
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [login]);

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
      <Text size="sm">{t("step1")}</Text>
      <Button
        component="a"
        href={login.verification_uri}
        target="_blank"
        rel="noopener noreferrer"
        color="violet"
        leftSection={<IconExternalLink size={18} />}
      >
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
