// 定义启动器信息类型
interface LauncherInfo {
  displayName: string;
  logoUrl?: string;
}

// 启动器信息映射表
export const LAUNCHER_INFO_MAP: Record<string, LauncherInfo> = {
  'zl': { displayName: 'ZalithLauncher' },
  'zl2': { displayName: 'ZalithLauncher2' },
  'hmcl': { displayName: 'Hello Minecraft! Launcher' },
  'MG': { displayName: 'MobileGlues' },
  'fcl': { displayName: 'FoldCraftLauncher' },
  'FCL_Turnip': { displayName: 'FCL_Turnip Plugin' },
  'shizuku': { displayName: 'Shizuku' },
  'leaves': { displayName: 'Leaves 服务端' },
  'leaf': { displayName: 'Leaf 服务端' },
  'luminol': { displayName: 'Luminol 服务端' }
};

/**
 * 获取启动器显示名称
 * @param name - 启动器名称
 * @returns 显示名称
 */
export const getLauncherDisplayName = (name: string): string => {
  return LAUNCHER_INFO_MAP[name]?.displayName || name;
};