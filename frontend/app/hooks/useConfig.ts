import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { ApiResponse } from "./useAxios";
import { AxiosInstance } from "axios";
import { User } from "./useAuthentication";

export interface Config {
  live_check_interval_seconds: number;
  video_check_interval_minutes: number;
  registration_enabled: boolean;
  parameters: {
    twitch_token: string;
    video_convert: string;
    chat_render: string;
    yt_dlp_video: string;
  };
  archive: {
    save_as_hls: boolean;
    generate_sprite_thumbnails: boolean;
    generate_nfo_files: boolean;
  };
  storage_templates: StorageTemplate;
  livestream: {
    proxies: ProxyListItem[];
    proxy_enabled: boolean;
    proxy_whitelist: string[];
    watch_while_archiving: boolean;
    split_duration_minutes: number;
    reconnect_grace_minutes: number;
  };
}

export interface StorageTemplate {
  folder_template: string;
  file_template: string;
  channel_folder_template: string;
}

export enum ProxyType {
  TwitchHLS = "twitch_hls",
  HTTP = "http",
}

export interface ProxyListItem {
  url: string;
  header: string;
  proxy_type: ProxyType;
}

const getConfig = async (axiosPrivate: AxiosInstance): Promise<Config> => {
  const response = await axiosPrivate.get<ApiResponse<Config>>(
    "/api/v1/config"
  );
  return response.data.data;
};

const useGetConfig = (axiosPrivate: AxiosInstance) => {
  return useQuery({
    queryKey: ["admin_config"],
    queryFn: () => getConfig(axiosPrivate),
  });
};

const editConfig = async (
  axiosPrivate: AxiosInstance,
  config: Config
): Promise<User> => {
  const response = await axiosPrivate.put(`/api/v1/config`, {
    ...config,
  });
  return response.data.data;
};

interface EditConfigVariables {
  axiosPrivate: AxiosInstance;
  config: Config;
}

const useEditConfig = () => {
  const queryClient = useQueryClient();
  return useMutation<User, Error, EditConfigVariables>({
    mutationFn: ({ axiosPrivate, config }) => editConfig(axiosPrivate, config),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["admin_config"] });
    },
  });
};

// A code the admin authorizes on Twitch to fill in the Twitch token.
export interface TwitchLogin {
  device_code: string;
  user_code: string;
  verification_uri: string;
  expires_in: number;
  interval: number;
}

export interface TwitchLoginPoll {
  status: "pending" | "authorized";
  twitch_token?: string;
}

const useStartTwitchLogin = () => {
  return useMutation<TwitchLogin, Error, AxiosInstance>({
    mutationFn: async (axiosPrivate) => {
      const response = await axiosPrivate.post<ApiResponse<TwitchLogin>>(`/api/v1/config/twitch-login`);
      return response.data.data;
    },
  });
};

interface PollTwitchLoginVariables {
  axiosPrivate: AxiosInstance;
  deviceCode: string;
}

const usePollTwitchLogin = () => {
  const queryClient = useQueryClient();
  return useMutation<TwitchLoginPoll, Error, PollTwitchLoginVariables>({
    mutationFn: async ({ axiosPrivate, deviceCode }) => {
      const response = await axiosPrivate.post<ApiResponse<TwitchLoginPoll>>(`/api/v1/config/twitch-login/poll`, {
        device_code: deviceCode,
      });
      return response.data.data;
    },
    onSuccess: (data) => {
      // The token is saved server-side; refetch later without resetting the settings form now.
      if (data.status === "authorized") {
        queryClient.invalidateQueries({ queryKey: ["admin_config"], refetchType: "none" });
      }
    },
  });
};

export { useEditConfig, useGetConfig, usePollTwitchLogin, useStartTwitchLogin };
