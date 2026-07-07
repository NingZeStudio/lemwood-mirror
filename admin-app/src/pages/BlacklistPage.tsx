import { useEffect, useMemo, useState } from 'react'
import {
  Table,
  Button,
  Form,
  Input,
  Popconfirm,
  message,
  Card,
  Space,
  Tag,
  Modal,
  Empty,
  Typography,
  Row,
  Col,
  Statistic,
  Flex,
  theme,
} from 'antd'
import {
  DeleteOutlined,
  PlusOutlined,
  SearchOutlined,
  StopOutlined,
  GlobalOutlined,
  RobotOutlined,
  UserOutlined,
  TeamOutlined,
  ExclamationCircleOutlined,
} from '@ant-design/icons'
import { getBlacklist, addBlacklist, removeBlacklist } from '@/api/blacklist'
import type { BlacklistItem } from '@/types'
import { useBreakpoint } from '@/hooks/useBreakpoint'
import dayjs from 'dayjs'

const { Title, Text } = Typography

type SourceType = 'manual' | 'external' | 'auto' | string

type FilterType = 'all' | SourceType

const SOURCE_META: Record<
  SourceType,
  { label: string; color: string; icon: React.ReactNode }
> = {
  manual: { label: '手动', color: 'blue', icon: <UserOutlined /> },
  external: { label: '外部同步', color: 'orange', icon: <GlobalOutlined /> },
  auto: { label: '自动封禁', color: 'red', icon: <RobotOutlined /> },
}

function getSourceMeta(source: SourceType) {
  return SOURCE_META[source] || { label: source || '未知', color: 'default', icon: <TeamOutlined /> }
}

export function BlacklistPage() {
  const [data, setData] = useState<BlacklistItem[]>([])
  const [loading, setLoading] = useState(false)
  const [modalOpen, setModalOpen] = useState(false)
  const [keyword, setKeyword] = useState('')
  const [filter, setFilter] = useState<FilterType>('all')
  const [form] = Form.useForm()
  const { isMobile } = useBreakpoint()
  const {
    token: { colorBgContainer, colorBorder },
  } = theme.useToken()

  useEffect(() => {
    loadBlacklist()
  }, [])

  async function loadBlacklist() {
    setLoading(true)
    try {
      const list = await getBlacklist()
      setData(list || [])
    } catch {
      message.error('加载黑名单失败')
    } finally {
      setLoading(false)
    }
  }

  const filteredData = useMemo(() => {
    return data.filter((item) => {
      const matchesFilter = filter === 'all' || item.source === filter
      const matchesKeyword =
        !keyword ||
        item.ip.toLowerCase().includes(keyword.toLowerCase()) ||
        item.reason.toLowerCase().includes(keyword.toLowerCase())
      return matchesFilter && matchesKeyword
    })
  }, [data, filter, keyword])

  const stats = useMemo(() => {
    return {
      total: data.length,
      manual: data.filter((i) => i.source === 'manual').length,
      external: data.filter((i) => i.source === 'external').length,
      auto: data.filter((i) => i.source === 'auto').length,
    }
  }, [data])

  const handleAdd = async (values: { ip: string; reason: string }) => {
    try {
      await addBlacklist(values)
      message.success('添加成功')
      form.resetFields()
      setModalOpen(false)
      loadBlacklist()
    } catch {
      message.error('添加失败')
    }
  }

  const handleRemove = async (ip: string) => {
    try {
      await removeBlacklist(ip)
      message.success('移除成功')
      loadBlacklist()
    } catch {
      message.error('移除失败')
    }
  }

  const columns = [
    {
      title: 'IP',
      dataIndex: 'ip',
      key: 'ip',
      width: 180,
      render: (ip: string) => <Text copyable={{ text: ip }}>{ip}</Text>,
    },
    {
      title: '来源',
      dataIndex: 'source',
      key: 'source',
      width: 120,
      render: (source: SourceType) => {
        const meta = getSourceMeta(source)
        return (
          <Tag icon={meta.icon} color={meta.color}>
            {meta.label}
          </Tag>
        )
      },
    },
    {
      title: '原因',
      dataIndex: 'reason',
      key: 'reason',
      ellipsis: true,
    },
    {
      title: '添加时间',
      dataIndex: 'created_at',
      key: 'created_at',
      width: 170,
      render: (time: string) => dayjs(time).format('YYYY-MM-DD HH:mm:ss'),
    },
    {
      title: '操作',
      key: 'action',
      width: 100,
      align: 'center' as const,
      render: (_: unknown, record: BlacklistItem) => (
        <Popconfirm
          title={`确定要移除 ${record.ip} 吗？`}
          onConfirm={() => handleRemove(record.ip)}
          okText="确定"
          cancelText="取消"
        >
          <Button type="link" danger icon={<DeleteOutlined />} size="small">
            移除
          </Button>
        </Popconfirm>
      ),
    },
  ]

  const renderMobileCard = (item: BlacklistItem) => {
    const meta = getSourceMeta(item.source)
    return (
      <Card
        key={item.ip}
        size="small"
        style={{ marginBottom: 12, background: colorBgContainer }}
        styles={{ body: { padding: 12 } }}
      >
        <Flex justify="space-between" align="start">
          <div style={{ minWidth: 0, flex: 1 }}>
            <Flex gap={8} align="center" wrap>
              <Text strong style={{ fontSize: 15 }}>
                {item.ip}
              </Text>
              <Tag icon={meta.icon} color={meta.color}>
                {meta.label}
              </Tag>
            </Flex>
            <div style={{ marginTop: 8, color: '#595959' }}>
              <Text type="secondary" style={{ fontSize: 13 }}>
                原因：
              </Text>
              <Text style={{ fontSize: 13 }}>{item.reason}</Text>
            </div>
            <div style={{ marginTop: 6 }}>
              <Text type="secondary" style={{ fontSize: 12 }}>
                {dayjs(item.created_at).format('YYYY-MM-DD HH:mm:ss')}
              </Text>
            </div>
          </div>
          <Popconfirm
            title={`确定要移除 ${item.ip} 吗？`}
            onConfirm={() => handleRemove(item.ip)}
            okText="确定"
            cancelText="取消"
          >
            <Button type="text" danger icon={<DeleteOutlined />} size="small" />
          </Popconfirm>
        </Flex>
      </Card>
    )
  }

  const statCards = [
    { key: 'all', title: '全部', value: stats.total, color: 'default' },
    { key: 'manual', title: '手动', value: stats.manual, color: 'blue' },
    { key: 'external', title: '外部', value: stats.external, color: 'orange' },
    { key: 'auto', title: '自动', value: stats.auto, color: 'red' },
  ]

  return (
    <div>
      <Title level={4} style={{ marginTop: 0, marginBottom: 16 }}>
        <StopOutlined style={{ marginRight: 8 }} />
        黑名单管理
      </Title>

      <Row gutter={[12, 12]} style={{ marginBottom: 16 }}>
        {statCards.map((s) => (
          <Col xs={12} sm={12} md={6} key={s.key}>
            <Card
              size="small"
              style={{
                cursor: 'pointer',
                borderColor: filter === s.key ? undefined : colorBorder,
              }}
              styles={{
                body: {
                  padding: isMobile ? 12 : 16,
                  background: filter === s.key ? '#e6f4ff' : undefined,
                },
              }}
              onClick={() => setFilter(s.key as FilterType)}
            >
              <Statistic
                title={s.title}
                value={s.value}
                valueStyle={{ color: s.color === 'default' ? '#262626' : undefined }}
              />
            </Card>
          </Col>
        ))}
      </Row>

      <Card
        style={{ marginBottom: 16 }}
        styles={{ body: { padding: isMobile ? 12 : 16 } }}
      >
        <Flex
          gap={12}
          vertical={isMobile}
          justify="space-between"
          align={isMobile ? 'stretch' : 'center'}
        >
          <Input
            placeholder="搜索 IP 或原因"
            prefix={<SearchOutlined />}
            value={keyword}
            onChange={(e) => setKeyword(e.target.value)}
            allowClear
            style={{ flex: 1, maxWidth: isMobile ? '100%' : 320 }}
          />
          <Button
            type="primary"
            icon={<PlusOutlined />}
            onClick={() => setModalOpen(true)}
            block={isMobile}
          >
            添加黑名单
          </Button>
        </Flex>
      </Card>

      {isMobile ? (
        <div>
          {filteredData.length === 0 ? (
            <Empty description="暂无数据" style={{ marginTop: 40 }} />
          ) : (
            filteredData.map(renderMobileCard)
          )}
        </div>
      ) : (
        <Table
          columns={columns}
          dataSource={filteredData}
          rowKey="ip"
          loading={loading}
          pagination={{
            pageSize: 10,
            showSizeChanger: true,
            pageSizeOptions: [10, 20, 50],
            showTotal: (total) => `共 ${total} 条`,
          }}
          scroll={{ x: 700 }}
          size="middle"
          locale={{ emptyText: <Empty description="暂无黑名单数据" /> }}
        />
      )}

      <Modal
        title="添加黑名单"
        open={modalOpen}
        onCancel={() => {
          setModalOpen(false)
          form.resetFields()
        }}
        footer={null}
        destroyOnClose
      >
        <Form form={form} layout="vertical" onFinish={handleAdd} style={{ marginTop: 16 }}>
          <Form.Item
            name="ip"
            label="IP 地址"
            rules={[{ required: true, message: '请输入 IP 地址' }]}
          >
            <Input placeholder="例如：192.168.1.1" />
          </Form.Item>
          <Form.Item
            name="reason"
            label="封禁原因"
            rules={[{ required: true, message: '请输入封禁原因' }]}
          >
            <Input.TextArea rows={3} placeholder="请输入封禁原因" />
          </Form.Item>
          <Form.Item style={{ marginBottom: 0 }}>
            <Space style={{ width: '100%', justifyContent: 'flex-end' }}>
              <Button
                onClick={() => {
                  setModalOpen(false)
                  form.resetFields()
                }}
              >
                取消
              </Button>
              <Button type="primary" htmlType="submit" icon={<PlusOutlined />}>
                添加
              </Button>
            </Space>
          </Form.Item>
        </Form>
      </Modal>

      {data.some((i) => i.source === 'external') && (
        <div
          style={{
            marginTop: 16,
            padding: 12,
            background: '#fffbe6',
            border: '1px solid #ffe58f',
            borderRadius: 8,
            display: 'flex',
            gap: 8,
            alignItems: 'flex-start',
          }}
        >
          <ExclamationCircleOutlined style={{ color: '#faad14', marginTop: 3 }} />
          <Text type="secondary" style={{ fontSize: 13 }}>
            来源为“外部同步”的记录由外部黑名单 URL 同步生成，移除后下次同步可能会重新出现。
          </Text>
        </div>
      )}
    </div>
  )
}
