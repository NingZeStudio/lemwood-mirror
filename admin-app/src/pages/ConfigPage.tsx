import { useState, useEffect } from 'react'
import {
  Form,
  Input,
  InputNumber,
  Switch,
  Button,
  Card,
  Divider,
  Select,
  message,
  Spin,
  Space,
  Image,
  Tag,
  Descriptions,
  Typography,
  Affix,
  Flex,
} from 'antd'
import {
  PlusOutlined,
  MinusCircleOutlined,
  SyncOutlined,
  CloudUploadOutlined,
  ReloadOutlined,
  SearchOutlined,
  SettingOutlined,
  SaveOutlined,
} from '@ant-design/icons'
import {
  getConfig,
  updateConfig,
  triggerLauncherScan,
  getSelfUpdateStatus,
  checkSelfUpdate,
  applySelfUpdate,
  restartSelfUpdate,
} from '@/api/config'
import type { Config, SelfUpdateStatus } from '@/types'
import { generateTOTPSecret, getTOTPQRCodeUrl } from '@/lib/utils'
import { useBreakpoint } from '@/hooks/useBreakpoint'

const { Title } = Typography

export function ConfigPage() {
  const [form] = Form.useForm()
  const [loading, setLoading] = useState(false)
  const [saving, setSaving] = useState(false)
  const [totpSecret, setTotpSecret] = useState('')
  const [scanningLauncher, setScanningLauncher] = useState<string | null>(null)
  const [selfUpdateStatus, setSelfUpdateStatus] = useState<SelfUpdateStatus | null>(null)
  const [checkingUpdate, setCheckingUpdate] = useState(false)
  const [applyingUpdate, setApplyingUpdate] = useState(false)
  const [restarting, setRestarting] = useState(false)
  const { isMobile } = useBreakpoint()

  async function loadConfig() {
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

  useEffect(() => {
    let cancelled = false
    ;(async () => {
      setLoading(true)
      try {
        const config = await getConfig()
        if (cancelled) return
        form.setFieldsValue({
          ...config,
          github_token: '',
          admin_password: '',
          captcha_secret_key: '',
        })
        setTotpSecret(config.two_factor_secret || '')
      } catch {
        if (!cancelled) message.error('加载配置失败')
      } finally {
        if (!cancelled) setLoading(false)
      }
    })()
    ;(async () => {
      try {
        const status = await getSelfUpdateStatus()
        if (!cancelled) setSelfUpdateStatus(status)
      } catch {
        // 自更新未启用时不报错
      }
    })()
    return () => {
      cancelled = true
    }
  }, [form])

  const handleSave = async (
    values: Config & { admin_password?: string; github_token?: string; captcha_secret_key?: string }
  ) => {
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

  const handleScanLauncher = async (launcherName: string) => {
    setScanningLauncher(launcherName)
    try {
      await triggerLauncherScan(launcherName)
      message.success(`已触发 ${launcherName} 的更新扫描`)
    } catch {
      message.error(`触发 ${launcherName} 更新扫描失败`)
    } finally {
      setScanningLauncher(null)
    }
  }

  const handleCheckUpdate = async () => {
    setCheckingUpdate(true)
    try {
      const status = await checkSelfUpdate()
      setSelfUpdateStatus(status)
      if (status.has_update) {
        message.success(`发现新版本 ${status.latest_version}`)
      } else {
        message.info('当前已是最新版本')
      }
    } catch {
      message.error('检查更新失败')
    } finally {
      setCheckingUpdate(false)
    }
  }

  const handleApplyUpdate = async () => {
    setApplyingUpdate(true)
    try {
      const status = await applySelfUpdate()
      setSelfUpdateStatus(status)
      if (status.pending_restart) {
        message.success('更新已应用，待重启生效')
      }
    } catch {
      message.error('应用更新失败')
    } finally {
      setApplyingUpdate(false)
    }
  }

  const handleRestart = async () => {
    setRestarting(true)
    try {
      await restartSelfUpdate()
      message.success('重启请求已发出')
    } catch {
      message.error('重启失败')
    } finally {
      setRestarting(false)
    }
  }

  if (loading) {
    return (
      <div style={{ textAlign: 'center', padding: 50 }}>
        <Spin size="large" />
      </div>
    )
  }

  const saveButton = (
    <Button type="primary" htmlType="submit" loading={saving} size="large" icon={<SaveOutlined />} block={isMobile}>
      保存配置
    </Button>
  )

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
      <Title level={4} style={{ marginTop: 0, marginBottom: 16 }}>
        <SettingOutlined style={{ marginRight: 8 }} />
        配置编辑
      </Title>

      <Card title="基础设置" style={{ marginBottom: 16 }}>
        <Form.Item label="管理员登录限制">
          <Space direction={isMobile ? 'vertical' : 'horizontal'} style={{ width: isMobile ? '100%' : 'auto' }}>
            <Form.Item name="admin_max_retries" noStyle>
              <InputNumber min={1} addonBefore="最大重试次数" style={{ width: isMobile ? '100%' : 170 }} />
            </Form.Item>
            <Form.Item name="admin_lock_duration" noStyle>
              <InputNumber min={1} addonBefore="锁定时长(分钟)" style={{ width: isMobile ? '100%' : 170 }} />
            </Form.Item>
          </Space>
        </Form.Item>
        <Form.Item name="server_port" label="服务端口" rules={[{ required: true }]}>
          <InputNumber min={1} max={65535} style={{ width: isMobile ? '100%' : 200 }} />
        </Form.Item>
        <Form.Item name="check_cron" label="检查频率 (Cron)" rules={[{ required: true }]} extra="例如: @every 10m">
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
        <Form.Item name="asset_proxy_url" label="资源代理前缀" extra="用于加速下载，例如: https://ghproxy.com/">
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
                    <Button onClick={handleGenerateTOTP} block={isMobile}>
                      生成新密钥
                    </Button>
                  </Space>
                </Form.Item>
                {totpSecret && (
                  <Form.Item label="二维码">
                    <Image src={getTOTPQRCodeUrl(totpSecret)} alt="TOTP QR Code" width={150} height={150} />
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

      <Card title="程序自更新" style={{ marginBottom: 16 }}>
        <Form.Item
          name="self_update_enabled"
          label="启用自更新检查"
          valuePropName="checked"
          extra="开启后可从 GitHub 检查程序自身更新"
        >
          <Switch />
        </Form.Item>
        <Form.Item
          name="self_update_repo_url"
          label="更新仓库地址"
          extra="例如: https://github.com/NingZeStudio/lemwood-mirror"
        >
          <Input placeholder="https://github.com/owner/repo" />
        </Form.Item>
        <Form.Item name="self_update_channel" label="更新类型" initialValue="notify">
          <Select
            options={[
              { label: '提醒 (仅提示)', value: 'notify' },
              { label: 'Release (稳定版)', value: 'release' },
              { label: 'Preview (预览版)', value: 'preview' },
            ]}
            style={{ width: isMobile ? '100%' : 200 }}
          />
        </Form.Item>
        <Form.Item
          name="self_update_check_cron"
          label="自动检查频率 (Cron)"
          extra="留空则仅手动检查。例如: @every 1h"
        >
          <Input placeholder="@every 1h" />
        </Form.Item>
        <Form.Item name="self_update_auto_restart" label="更新后自动重启" valuePropName="checked">
          <Switch />
        </Form.Item>

        {selfUpdateStatus && selfUpdateStatus.enabled && (
          <Card size="small" style={{ background: '#fafafa', marginTop: 12 }}>
            <Descriptions column={isMobile ? 1 : 2} size="small">
              <Descriptions.Item label="当前版本">
                <Tag color="blue">{selfUpdateStatus.current_version}</Tag>
              </Descriptions.Item>
              <Descriptions.Item label="远端版本">
                <Tag color={selfUpdateStatus.has_update ? 'green' : 'default'}>
                  {selfUpdateStatus.latest_version || '-'}
                </Tag>
              </Descriptions.Item>
              <Descriptions.Item label="最近检查">
                {selfUpdateStatus.last_checked_at ? new Date(selfUpdateStatus.last_checked_at).toLocaleString() : '-'}
              </Descriptions.Item>
              <Descriptions.Item label="状态">
                {selfUpdateStatus.pending_restart ? (
                  <Tag color="orange">待重启</Tag>
                ) : selfUpdateStatus.has_update ? (
                  <Tag color="green">有更新</Tag>
                ) : (
                  <Tag>已最新</Tag>
                )}
              </Descriptions.Item>
              {selfUpdateStatus.last_check_error && (
                <Descriptions.Item label="检查错误" span={2}>
                  <span style={{ color: 'red' }}>{selfUpdateStatus.last_check_error}</span>
                </Descriptions.Item>
              )}
              {selfUpdateStatus.last_apply_error && (
                <Descriptions.Item label="更新错误" span={2}>
                  <span style={{ color: 'red' }}>{selfUpdateStatus.last_apply_error}</span>
                </Descriptions.Item>
              )}
              {selfUpdateStatus.last_apply_message && (
                <Descriptions.Item label="更新信息" span={2}>
                  {selfUpdateStatus.last_apply_message}
                </Descriptions.Item>
              )}
            </Descriptions>
            <Flex gap={8} wrap style={{ marginTop: 12 }}>
              <Button icon={<SearchOutlined />} loading={checkingUpdate} onClick={handleCheckUpdate}>
                检查更新
              </Button>
              <Button
                icon={<CloudUploadOutlined />}
                loading={applyingUpdate}
                disabled={!selfUpdateStatus.can_apply}
                onClick={handleApplyUpdate}
              >
                应用更新
              </Button>
              <Button icon={<ReloadOutlined />} loading={restarting} danger onClick={handleRestart}>
                重启
              </Button>
            </Flex>
          </Card>
        )}
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
                form.setFieldValue('launchers', [
                  ...launchers,
                  { name: '', source_url: '', repo_selector: '', mode: 'release' },
                ])
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
                      <Button type="text" danger icon={<MinusCircleOutlined />} onClick={() => remove(name)}>
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
                  <Form.Item {...restField} name={[name, 'repo_selector']} label="版本选择器 (可选)">
                    <Input />
                  </Form.Item>
                  <Form.Item
                    {...restField}
                    name={[name, 'mode']}
                    label="同步模式"
                    initialValue="release"
                    extra="release: 仅同步 Release；clone: 仅同步 Git 仓库；all: 两者都同步"
                  >
                    <Select
                      options={[
                        { label: 'release', value: 'release' },
                        { label: 'clone', value: 'clone' },
                        { label: 'all', value: 'all' },
                      ]}
                    />
                  </Form.Item>
                  <Form.Item noStyle shouldUpdate>
                    {({ getFieldValue }) => {
                      const launcherName = getFieldValue(['launchers', name, 'name'])
                      const mode = getFieldValue(['launchers', name, 'mode']) || 'release'
                      if (mode !== 'clone' && mode !== 'all') {
                        return null
                      }
                      return (
                        <div style={{ marginBottom: 16, color: '#666' }}>
                          Git 克隆地址：<code>/repo/{launcherName || '{launcher}'}.git</code>
                        </div>
                      )
                    }}
                  </Form.Item>
                  <Form.Item
                    {...restField}
                    name={[name, 'max_versions']}
                    label="最大版本数"
                    extra="0 表示使用默认值 3，同时最多保留最近 3 个版本"
                  >
                    <InputNumber min={0} max={50} style={{ width: '100%' }} />
                  </Form.Item>
                  <Form.Item
                    {...restField}
                    name={[name, 'include_prerelease']}
                    label="包含预发布版本"
                    valuePropName="checked"
                  >
                    <Switch />
                  </Form.Item>
                  <Form.Item noStyle shouldUpdate>
                    {({ getFieldValue }) => {
                      const launcherName = getFieldValue(['launchers', name, 'name'])
                      return (
                        <Button
                          type="default"
                          icon={<SyncOutlined spin={scanningLauncher === launcherName} />}
                          onClick={() => handleScanLauncher(launcherName)}
                          disabled={!launcherName || scanningLauncher !== null}
                          loading={scanningLauncher === launcherName}
                          block={isMobile}
                        >
                          请求更新
                        </Button>
                      )
                    }}
                  </Form.Item>
                </Card>
              ))}
            </>
          )}
        </Form.List>
      </Card>

      <Divider />
      {isMobile ? (
        <Affix offsetBottom={16}>
          <div
            style={{
              padding: '12px 0',
              background: '#fff',
              borderTop: '1px solid #f0f0f0',
            }}
          >
            {saveButton}
          </div>
        </Affix>
      ) : (
        <Form.Item>{saveButton}</Form.Item>
      )}
    </Form>
  )
}
