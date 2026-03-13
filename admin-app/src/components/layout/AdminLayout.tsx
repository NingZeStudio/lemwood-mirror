import { useState } from 'react'
import { Layout, Menu, Button, Drawer, theme } from 'antd'
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

const menuItems = [
  {
    key: '/config',
    icon: <SettingOutlined />,
    label: '配置编辑',
  },
  {
    key: '/files',
    icon: <FolderOutlined />,
    label: '文件管理',
  },
  {
    key: '/blacklist',
    icon: <StopOutlined />,
    label: '黑名单管理',
  },
]

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
        items={menuItems}
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

  if (isMobile) {
    return (
      <Layout style={{ minHeight: '100vh' }}>
        <div
          style={{
            padding: '0 16px',
            background: colorBgContainer,
            display: 'flex',
            alignItems: 'center',
            height: 56,
            borderBottom: '1px solid #f0f0f0',
            position: 'sticky',
            top: 0,
            zIndex: 100,
          }}
        >
          <Button
            type="text"
            icon={<MenuOutlined />}
            onClick={() => setDrawerOpen(true)}
            style={{ fontSize: 18, width: 48, height: 48 }}
          />
          <span style={{ fontSize: 16, fontWeight: 500, marginLeft: 8 }}>
            后台管理
          </span>
        </div>
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
            margin: 16,
            padding: 16,
            background: colorBgContainer,
            borderRadius: borderRadiusLG,
            minHeight: 'calc(100vh - 88px)',
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
        <div
          style={{
            padding: '0 16px',
            background: colorBgContainer,
            display: 'flex',
            alignItems: 'center',
            height: 64,
            borderBottom: '1px solid #f0f0f0',
          }}
        >
          <Button
            type="text"
            icon={collapsed ? <MenuUnfoldOutlined /> : <MenuFoldOutlined />}
            onClick={() => setCollapsed(!collapsed)}
            style={{ fontSize: 16, width: 64, height: 64 }}
          />
          <span style={{ fontSize: 16, fontWeight: 500, marginLeft: 8 }}>
            后台管理
          </span>
        </div>
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
