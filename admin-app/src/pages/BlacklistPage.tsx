import { useState, useEffect } from 'react'
import { Table, Button, Form, Input, Popconfirm, message, Card, Space } from 'antd'
import { DeleteOutlined, PlusOutlined } from '@ant-design/icons'
import { getBlacklist, addBlacklist, removeBlacklist } from '@/api/blacklist'
import type { BlacklistItem } from '@/types'
import { useBreakpoint } from '@/hooks/useBreakpoint'
import dayjs from 'dayjs'

export function BlacklistPage() {
  const [data, setData] = useState<BlacklistItem[]>([])
  const [loading, setLoading] = useState(false)
  const [form] = Form.useForm()
  const { isMobile } = useBreakpoint()

  useEffect(() => {
    loadBlacklist()
  }, [])

  const loadBlacklist = async () => {
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

  const handleAdd = async (values: { ip: string; reason: string }) => {
    try {
      await addBlacklist(values)
      message.success('添加成功')
      form.resetFields()
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
      width: isMobile ? 120 : 180,
    },
    ...(isMobile ? [] : [
      {
        title: '原因',
        dataIndex: 'reason',
        key: 'reason',
      },
      {
        title: '添加时间',
        dataIndex: 'created_at',
        key: 'created_at',
        width: 160,
        render: (time: string) => dayjs(time).format('YYYY-MM-DD HH:mm'),
      },
    ]),
    {
      title: '操作',
      key: 'action',
      width: isMobile ? 60 : 100,
      render: (_: unknown, record: BlacklistItem) => (
        <Popconfirm
          title={`确定要移除 ${record.ip} 吗？`}
          onConfirm={() => handleRemove(record.ip)}
          okText="确定"
          cancelText="取消"
        >
          <Button type="link" danger icon={<DeleteOutlined />} size="small">
            {!isMobile && '移除'}
          </Button>
        </Popconfirm>
      ),
    },
  ]

  return (
    <div>
      <Card style={{ marginBottom: 16 }}>
        <Form 
          form={form} 
          layout={isMobile ? 'vertical' : 'inline'} 
          onFinish={handleAdd}
        >
          <Form.Item
            name="ip"
            rules={[{ required: true, message: '请输入 IP 地址' }]}
            style={isMobile ? { marginBottom: 12 } : undefined}
          >
            <Input 
              placeholder="IP 地址" 
              style={{ width: isMobile ? '100%' : 180 }} 
            />
          </Form.Item>
          <Form.Item
            name="reason"
            rules={[{ required: true, message: '请输入原因' }]}
            style={isMobile ? { marginBottom: 12 } : undefined}
          >
            <Input 
              placeholder="原因" 
              style={{ width: isMobile ? '100%' : 250 }} 
            />
          </Form.Item>
          <Form.Item style={isMobile ? { marginBottom: 0 } : undefined}>
            <Button 
              type="primary" 
              htmlType="submit" 
              icon={<PlusOutlined />} 
              block={isMobile}
            >
              添加
            </Button>
          </Form.Item>
        </Form>
      </Card>

      <Table
        columns={columns}
        dataSource={data}
        rowKey="ip"
        loading={loading}
        pagination={false}
        scroll={{ x: isMobile ? 250 : undefined }}
        size={isMobile ? 'small' : 'middle'}
        expandable={isMobile ? {
          expandedRowRender: (record) => (
            <Space direction="vertical" size="small">
              <div><strong>原因：</strong>{record.reason}</div>
              <div><strong>添加时间：</strong>{dayjs(record.created_at).format('YYYY-MM-DD HH:mm:ss')}</div>
            </Space>
          ),
        } : undefined}
      />
    </div>
  )
}
