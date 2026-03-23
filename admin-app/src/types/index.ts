export interface LoginRequest {
  username: string
  password: string
  otp_code?: string
}

export interface LoginResponse {
  token: string
}

export interface TOTPStatus {
  enabled: boolean
}

export interface LauncherConfig {
  name: string
  source_url: string
  repo_selector: string
  include_prerelease?: boolean
  max_versions?: number
}

export interface Config {
  server_port: number
  check_cron: string
  storage_path: string
  download_url_base: string
  admin_user: string
  admin_enabled: boolean
  admin_max_retries: number
  admin_lock_duration: number
  proxy_url: string
  asset_proxy_url: string
  github_token?: string
  concurrent_downloads: number
  download_timeout_minutes: number
  xget_enabled: boolean
  xget_domain: string
  two_factor_enabled: boolean
  two_factor_secret: string
  captcha_enabled: boolean
  captcha_app_id: string
  captcha_secret_key?: string
  launchers: LauncherConfig[]
}

export interface ConfigUpdateRequest extends Partial<Config> {
  admin_password?: string
}

export interface FileInfo {
  name: string
  is_dir: boolean
  size: number
  mod_time: string
}

export interface BlacklistItem {
  ip: string
  reason: string
  created_at: string
}

export interface AddBlacklistRequest {
  ip: string
  reason: string
}
