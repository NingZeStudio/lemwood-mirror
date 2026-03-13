import { useState, useEffect } from 'react'
import { Form, Input, Button, Card, message } from 'antd'
import { UserOutlined, LockOutlined, SafetyOutlined } from '@ant-design/icons'
import { useNavigate } from 'react-router-dom'
import { login, getTOTPStatus } from '@/api/auth'
import { useAuthStore } from '@/store/authStore'
import { useBreakpoint } from '@/hooks/useBreakpoint'

interface LoginForm {
  username: string
  password: string
  otp_code?: string
}

export function LoginPage() {
  const [loading, setLoading] = useState(false)
  const [totpEnabled, setTotpEnabled] = useState(false)
  const navigate = useNavigate()
  const setToken = useAuthStore((state) => state.setToken)
  const { isMobile } = useBreakpoint()

  useEffect(() => {
    const checkTOTP = async () => {
      try {
        const status = await getTOTPStatus()
        setTotpEnabled(status.enabled)
      } catch {
        // ignore
      }
    }
    checkTOTP()
  }, [])

  const handleSubmit = async (values: LoginForm) => {
    setLoading(true)
    try {
      const response = await login(values)
      setToken(response.token)
      message.success('登录成功')
      navigate('/config')
    } catch (error: unknown) {
      const err = error as { response?: { data?: string } }
      message.error(err.response?.data || '登录失败，请检查用户名、密码或验证码')
    } finally {
      setLoading(false)
    }
  }

  return (
    <div
      style={{
        minHeight: '100vh',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        background: '#f0f2f5',
        padding: isMobile ? 16 : 0,
      }}
    >
      <Card
        title={
          <div style={{ textAlign: 'center', fontSize: isMobile ? 18 : 20 }}>
            Lemwood Mirror 后台登录
          </div>
        }
        style={{ 
          width: isMobile ? '100%' : 400,
          maxWidth: '100%',
        }}
      >
        <Form
          name="login"
          onFinish={handleSubmit}
          autoComplete="off"
          layout="vertical"
        >
          <Form.Item
            name="username"
            rules={[{ required: true, message: '请输入用户名' }]}
          >
            <Input
              prefix={<UserOutlined />}
              placeholder="用户名"
              size="large"
            />
          </Form.Item>

          <Form.Item
            name="password"
            rules={[{ required: true, message: '请输入密码' }]}
          >
            <Input.Password
              prefix={<LockOutlined />}
              placeholder="密码"
              size="large"
            />
          </Form.Item>

          {totpEnabled && (
            <Form.Item
              name="otp_code"
              rules={[
                { required: true, message: '请输入验证码' },
                { pattern: /^\d{6}$/, message: '请输入6位数字验证码' },
              ]}
            >
              <Input
                prefix={<SafetyOutlined />}
                placeholder="6 位验证码"
                size="large"
                maxLength={6}
              />
            </Form.Item>
          )}

          <Form.Item>
            <Button
              type="primary"
              htmlType="submit"
              loading={loading}
              size="large"
              block
            >
              登录
            </Button>
          </Form.Item>
        </Form>
      </Card>
    </div>
  )
}
