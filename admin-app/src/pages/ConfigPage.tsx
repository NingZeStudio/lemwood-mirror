import { useState, useEffect } from 'react'
import {
  Form,
  Input,
  InputNumber,
  Switch,
  Button,
  Card,
  Divider,
  message,
  Spin,
  Space,
  Image,
} from 'antd'
import { PlusOutlined, MinusCircleOutlined } from '@ant-design/icons'
import { getConfig, updateConfig } from '@/api/config'
import type { Config } from '@/types'
import { generateTOTPSecret, getTOTPQRCodeUrl } from '@/lib/utils'
import { useBreakpoint } from '@/hooks/useBreakpoint'

export function ConfigPage() {
  const [form] = Form.useForm()
  const [loading, setLoading] = useState(false)
  const [saving, setSaving] = useState(false)
  const [totpSecret, setTotpSecret] = useState('')
  const { isMobile } = useBreakpoint()

  useEffect(() => {
    loadConfig()
  }, [])

  const loadConfig = async () => {
    setLoading(true)
    try {
      const config = await getConfig()
      form.setFieldsValue({
        ...config,
        github_token: '',
        admin_password: '',
        captcha_secret_key: '',
      })
      setTotpSecret(config.two_factor_secret || '')
    } catch {
      message.error('加载配置失败')
    } finally {
      setLoading(false)
    }
  }

  const handleSave = async (values: Config & { admin_password?: string; github_token?: string; captcha_secret_key?: string }) => {
    setSaving(true)
    try {
      const updateData: Record<string, unknown> = { ...values }
      
      if (!values.admin_password) {
        delete updateData.admin_password
      }
      if (!values.github_token) {
        delete updateData.github_token
      }
      if (!values.captcha_secret_key) {
        delete updateData.captcha_secret_key
      }

      await updateConfig(updateData)
      message.success('保存成功，部分配置可能需要重启生效')
      loadConfig()
    } catch (error: unknown) {
      const err = error as { response?: { data?: string } }
      message.error(err.response?.data || '保存失败')
    } finally {
      setSaving(false)
    }
  }

  const handleGenerateTOTP = () => {
    const secret = generateTOTPSecret()
    setTotpSecret(secret)
    form.setFieldValue('two_factor_secret', secret)
  }

  const handleTOTPEnabledChange = (checked: boolean) => {
    if (checked && !totpSecret) {
      handleGenerateTOTP()
    }
  }

  if (loading) {
    return (
      <div style={{ textAlign: 'center', padding: 50 }}>
        <Spin size="large" />
      </div>
    )
  }

  return (
    <Form
      form={form}
      layout="vertical"
      onFinish={handleSave}
      initialValues={{
        admin_max_retries: 10,
        admin_lock_duration: 120,
        concurrent_downloads: 3,
        download_timeout_minutes: 30,
      }}
    >
      <Card title="基础设置" style={{ marginBottom: 16 }}>
        <Form.Item label="管理员登录限制">
          <Space direction={isMobile ? 'vertical' : 'horizontal'} style={{ width: isMobile ? '100%' : 'auto' }}>
            <Form.Item name="admin_max_retries" noStyle>
              <InputNumber 
                min={1} 
                addonBefore="最大重试次数" 
                style={{ width: isMobile ? '100%' : 150 }} 
              />
            </Form.Item>
            <Form.Item name="admin_lock_duration" noStyle>
              <InputNumber 
                min={1} 
                addonBefore="锁定时长(分钟)" 
                style={{ width: isMobile ? '100%' : 150 }} 
              />
            </Form.Item>
          </Space>
        </Form.Item>
        <Form.Item name="server_port" label="服务端口" rules={[{ required: true }]}>
          <InputNumber min={1} max={65535} style={{ width: isMobile ? '100%' : 200 }} />
        </Form.Item>
        <Form.Item
          name="check_cron"
          label="检查频率 (Cron)"
          rules={[{ required: true }]}
          extra="例如: @every 10m"
        >
          <Input placeholder="@every 10m" />
        </Form.Item>
        <Form.Item name="storage_path" label="存储路径" rules={[{ required: true }]}>
          <Input />
        </Form.Item>
        <Form.Item
          name="download_url_base"
          label="下载链接前缀"
          rules={[{ required: true }]}
          extra="例如: https://mirror.example.com"
        >
          <Input />
        </Form.Item>
      </Card>

      <Card title="GitHub 配置" style={{ marginBottom: 16 }}>
        <Form.Item name="github_token" label="GitHub Token" extra="留空不修改">
          <Input.Password placeholder="用于 API 认证，避免限流" />
        </Form.Item>
        <Form.Item name="proxy_url" label="代理 URL" extra="例如: http://127.0.0.1:7890">
          <Input />
        </Form.Item>
        <Form.Item
          name="asset_proxy_url"
          label="资源代理前缀"
          extra="用于加速下载，例如: https://ghproxy.com/"
        >
          <Input />
        </Form.Item>
      </Card>

      <Card title="管理员设置" style={{ marginBottom: 16 }}>
        <Form.Item name="admin_enabled" label="启用管理后台" valuePropName="checked">
          <Switch />
        </Form.Item>
        <Form.Item name="admin_user" label="管理员账号" rules={[{ required: true }]}>
          <Input />
        </Form.Item>
        <Form.Item name="admin_password" label="新密码" extra="留空不修改">
          <Input.Password />
        </Form.Item>
      </Card>

      <Card title="二次验证 (TOTP)" style={{ marginBottom: 16 }}>
        <Form.Item
          name="two_factor_enabled"
          label="启用两步验证"
          valuePropName="checked"
          extra="使用微软/谷歌验证器"
        >
          <Switch onChange={handleTOTPEnabledChange} />
        </Form.Item>
        <Form.Item noStyle shouldUpdate>
          {({ getFieldValue }) =>
            getFieldValue('two_factor_enabled') && (
              <>
                <Form.Item name="two_factor_secret" label="验证器密钥 (Secret)">
                  <Space direction={isMobile ? 'vertical' : 'horizontal'} style={{ width: isMobile ? '100%' : 'auto' }}>
                    <Input readOnly style={{ width: isMobile ? '100%' : 200 }} />
                    <Button onClick={handleGenerateTOTP} block={isMobile}>生成新密钥</Button>
                  </Space>
                </Form.Item>
                {totpSecret && (
                  <Form.Item label="二维码">
                    <Image
                      src={getTOTPQRCodeUrl(totpSecret)}
                      alt="TOTP QR Code"
                      width={150}
                      height={150}
                    />
                    <div style={{ color: '#666', marginTop: 8 }}>
                      请将此密钥手动输入或扫描二维码添加至您的验证器应用
                    </div>
                  </Form.Item>
                )}
              </>
            )
          }
        </Form.Item>
      </Card>

      <Card title="腾讯云验证码" style={{ marginBottom: 16 }}>
        <Form.Item name="captcha_enabled" label="启用下载验证" valuePropName="checked">
          <Switch />
        </Form.Item>
        <Form.Item name="captcha_app_id" label="Captcha ID">
          <Input placeholder="极验验证码 Captcha ID" />
        </Form.Item>
        <Form.Item name="captcha_secret_key" label="Private Key" extra="留空不修改">
          <Input.Password placeholder="服务端验证密钥" />
        </Form.Item>
      </Card>

      <Card title="高级设置" style={{ marginBottom: 16 }}>
        <Form.Item name="concurrent_downloads" label="并发下载数">
          <InputNumber min={1} max={10} style={{ width: isMobile ? '100%' : 200 }} />
        </Form.Item>
        <Form.Item name="download_timeout_minutes" label="下载超时 (分钟)">
          <InputNumber min={1} style={{ width: isMobile ? '100%' : 200 }} />
        </Form.Item>
        <Form.Item name="xget_enabled" label="启用 Xget 镜像加速" valuePropName="checked">
          <Switch />
        </Form.Item>
        <Form.Item name="xget_domain" label="Xget 域名">
          <Input />
        </Form.Item>
      </Card>

      <Card 
        title="启动器配置" 
        style={{ marginBottom: 16 }}
        extra={
          !isMobile && (
            <Button
              type="primary"
              icon={<PlusOutlined />}
              onClick={() => {
                const launchers = form.getFieldValue('launchers') || []
                form.setFieldValue('launchers', [...launchers, { name: '', source_url: '', repo_selector: '' }])
              }}
            >
              添加启动器
            </Button>
          )
        }
      >
        {isMobile && (
          <Button
            type="primary"
            icon={<PlusOutlined />}
            onClick={() => {
              const launchers = form.getFieldValue('launchers') || []
              form.setFieldValue('launchers', [...launchers, { name: '', source_url: '', repo_selector: '' }])
            }}
            style={{ marginBottom: 16 }}
            block
          >
            添加启动器
          </Button>
        )}
        <Form.List name="launchers">
          {(fields, { remove }) => (
            <>
              {fields.map(({ key, name, ...restField }) => (
                <Card
                  key={key}
                  size="small"
                  style={{ marginBottom: 16 }}
                  extra={
                    !isMobile && (
                      <Button
                        type="text"
                        danger
                        icon={<MinusCircleOutlined />}
                        onClick={() => remove(name)}
                      >
                        删除
                      </Button>
                    )
                  }
                >
                  {isMobile && (
                    <Button
                      type="text"
                      danger
                      icon={<MinusCircleOutlined />}
                      onClick={() => remove(name)}
                      style={{ marginBottom: 8 }}
                    >
                      删除
                    </Button>
                  )}
                  <Form.Item
                    {...restField}
                    name={[name, 'name']}
                    label="名称 (如 fcl, zl)"
                    rules={[{ required: true }]}
                  >
                    <Input />
                  </Form.Item>
                  <Form.Item
                    {...restField}
                    name={[name, 'source_url']}
                    label="GitHub 仓库 URL / 来源页面"
                    rules={[{ required: true }]}
                  >
                    <Input />
                  </Form.Item>
                  <Form.Item
                    {...restField}
                    name={[name, 'repo_selector']}
                    label="版本选择器 (可选)"
                  >
                    <Input />
                  </Form.Item>
                </Card>
              ))}
            </>
          )}
        </Form.List>
      </Card>

      <Divider />
      <Form.Item>
        <Button type="primary" htmlType="submit" loading={saving} size="large" block={isMobile}>
          保存配置
        </Button>
      </Form.Item>
    </Form>
  )
}
