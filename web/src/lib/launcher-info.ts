import { globalConfig } from '@/lib/globalConfig'

export const LAUNCHER_INFO_MAP: Record<string, { displayName: string; logoUrl?: string }> =
  globalConfig.launchers as Record<string, { displayName: string; logoUrl?: string }>

export function getLauncherDisplayName(name: string): string {
  const launchers = globalConfig.launchers as Record<string, { displayName: string; logoUrl?: string }>
  return launchers[name]?.displayName || name
}
