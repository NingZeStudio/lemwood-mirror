import { Grid } from 'antd'

const { useBreakpoint: useAntBreakpoint } = Grid

export function useBreakpoint() {
  const screens = useAntBreakpoint()
  
  const isMobile = !screens.md
  const isTablet = screens.md && !screens.lg
  const isDesktop = screens.lg || false
  
  return {
    screens,
    isMobile,
    isTablet,
    isDesktop,
  }
}
