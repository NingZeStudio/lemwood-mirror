import { useState, useMemo } from 'react'
import { Layout, Menu, Button, Drawer, theme, Typography, Space, Badge } from 'antd'
import {
  SettingOutlined,
  FolderOutlined,
  StopOutlined,
  LogoutOutlined,
  MenuFoldOutlined,
  MenuUnfoldOutlined,
  MenuOutlined,
} from '@ant-design/icons'
import { useNavigate, useLocation, Outlet } from 'react-router-dom'
import { useAuthStore } from '@/store/authStore'
import { useBreakpoint } from '@/hooks/useBreakpoint'

const { Sider, Content } = Layout
const { Title } = Typography

interface MenuItem {
  key: string
  icon: React.ReactNode
  label: string
  title: string
}

const menuItems: MenuItem[] = [
  {
    key: '/config',
    icon: <SettingOutlined />,
    label: '配置编辑',
    title: '配置编辑',
  },
  {
    key: '/files',
    icon: <FolderOutlined />,
    label: '文件管理',
    title: '文件管理',
  },
  {
    key: '/blacklist',
    icon: <StopOutlined />,
    label: '黑名单管理',
    title: '黑名单管理',
  },
]

const menuNavItems = menuItems.map(({ key, icon, label }) => ({ key, icon, label }))

export function AdminLayout() {
  const [collapsed, setCollapsed] = useState(false)
  const [drawerOpen, setDrawerOpen] = useState(false)
  const navigate = useNavigate()
  const location = useLocation()
  const logout = useAuthStore((state) => state.logout)
  const { isMobile } = useBreakpoint()
  const {
    token: { colorBgContainer, borderRadiusLG },
  } = theme.useToken()

  const activeItem = useMemo(
    () => menuItems.find((item) => item.key === location.pathname) || menuItems[0],
    [location.pathname]
  )

  const handleMenuClick = ({ key }: { key: string }) => {
    navigate(key)
    if (isMobile) {
      setDrawerOpen(false)
    }
  }

  const handleLogout = () => {
    logout()
    navigate('/')
  }

  const menuContent = (
    <>
      <Menu
        mode="inline"
        selectedKeys={[location.pathname]}
        items={menuNavItems}
        onClick={handleMenuClick}
        style={{ borderRight: 0 }}
      />
      <div
        style={{
          position: 'absolute',
          bottom: 0,
          width: '100%',
          padding: 16,
          borderTop: '1px solid #f0f0f0',
        }}
      >
        <Button
          type="text"
          icon={<LogoutOutlined />}
          onClick={handleLogout}
          style={{ width: '100%', textAlign: collapsed && !isMobile ? 'center' : 'left' }}
        >
          {(!collapsed || isMobile) && '退出登录'}
        </Button>
      </div>
    </>
  )

  const logoContent = (
    <div
      style={{
        height: 64,
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        borderBottom: '1px solid #f0f0f0',
      }}
    >
      <span
        style={{
          fontSize: collapsed && !isMobile ? 14 : 16,
          fontWeight: 600,
          whiteSpace: 'nowrap',
          overflow: 'hidden',
        }}
      >
        {collapsed && !isMobile ? 'LM' : 'Lemwood Mirror'}
      </span>
    </div>
  )

  const headerBar = (
    <div
      style={{
        padding: '0 16px',
        background: colorBgContainer,
        display: 'flex',
        alignItems: 'center',
        height: isMobile ? 56 : 64,
        borderBottom: '1px solid #f0f0f0',
        position: 'sticky',
        top: 0,
        zIndex: 100,
      }}
    >
      <Button
        type="text"
        icon={isMobile ? <MenuOutlined /> : collapsed ? <MenuUnfoldOutlined /> : <MenuFoldOutlined />}
        onClick={() => (isMobile ? setDrawerOpen(true) : setCollapsed(!collapsed))}
        style={{ fontSize: isMobile ? 18 : 16, width: isMobile ? 48 : 64, height: isMobile ? 48 : 64 }}
      />
      <Space direction="vertical" size={0} style={{ marginLeft: 8, flex: 1, minWidth: 0 }}>
        <Title level={5} style={{ margin: 0, fontSize: isMobile ? 15 : 16, lineHeight: '22px' }}>
          {activeItem.title}
        </Title>
        {!isMobile && (
          <span style={{ fontSize: 12, color: '#8c8c8c', lineHeight: '18px' }}>Lemwood Mirror 后台管理</span>
        )}
      </Space>
      {!isMobile && (
        <Badge dot color="green">
          <span style={{ fontSize: 13, color: '#595959' }}>在线</span>
        </Badge>
      )}
    </div>
  )

  if (isMobile) {
    return (
      <Layout style={{ minHeight: '100vh' }}>
        {headerBar}
        <Drawer
          placement="left"
          onClose={() => setDrawerOpen(false)}
          open={drawerOpen}
          width={250}
          styles={{ body: { padding: 0, position: 'relative' } }}
        >
          {logoContent}
          {menuContent}
        </Drawer>
        <Content
          style={{
            margin: 12,
            padding: 16,
            background: colorBgContainer,
            borderRadius: borderRadiusLG,
            minHeight: 'calc(100vh - 80px)',
          }}
        >
          <Outlet />
        </Content>
      </Layout>
    )
  }

  return (
    <Layout style={{ minHeight: '100vh' }}>
      <Sider trigger={null} collapsible collapsed={collapsed} theme="light">
        {logoContent}
        {menuContent}
      </Sider>
      <Layout>
        {headerBar}
        <Content
          style={{
            margin: 24,
            padding: 24,
            background: colorBgContainer,
            borderRadius: borderRadiusLG,
            minHeight: 280,
          }}
        >
          <Outlet />
        </Content>
      </Layout>
    </Layout>
  )
}
