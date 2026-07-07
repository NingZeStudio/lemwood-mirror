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
  mode?: 'release' | 'clone' | 'all'
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
  self_update_enabled?: boolean
  self_update_repo_url?: string
  self_update_channel?: string
  self_update_check_cron?: string
  self_update_auto_restart?: boolean
}

export interface ConfigUpdateRequest extends Partial<Config> {
  admin_password?: string
}

export interface TagInfo {
  name: string
  stable: boolean
}

export interface SelfUpdateStatus {
  enabled: boolean
  repo_url: string
  channel: string
  current_version: string
  latest_version: string
  has_update: boolean
  can_apply: boolean
  pending_restart: boolean
  last_checked_at: string
  last_applied_at: string
  last_check_error: string
  last_apply_error: string
  last_apply_message: string
  available_versions: TagInfo[]
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
  source: string
  ban_type: string
  created_at: string
}

export interface AddBlacklistRequest {
  ip: string
  reason: string
}
