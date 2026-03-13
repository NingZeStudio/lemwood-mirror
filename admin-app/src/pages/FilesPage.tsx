import { useState, useEffect } from 'react'
import {
  Table,
  Button,
  Breadcrumb,
  Popconfirm,
  Upload,
  message,
  Empty,
  Space,
} from 'antd'
import {
  FolderOutlined,
  FileOutlined,
  DownloadOutlined,
  DeleteOutlined,
  UploadOutlined,
  HomeOutlined,
} from '@ant-design/icons'
import type { UploadProps } from 'antd'
import { getFiles, deleteFile, downloadFile, uploadFile } from '@/api/files'
import type { FileInfo } from '@/types'
import { formatSize } from '@/lib/utils'
import { useBreakpoint } from '@/hooks/useBreakpoint'
import dayjs from 'dayjs'

export function FilesPage() {
  const [files, setFiles] = useState<FileInfo[]>([])
  const [loading, setLoading] = useState(false)
  const [currentPath, setCurrentPath] = useState('')
  const { isMobile } = useBreakpoint()

  useEffect(() => {
    loadFiles('')
  }, [])

  const loadFiles = async (path: string) => {
    setLoading(true)
    try {
      const data = await getFiles(path)
      setFiles(data)
      setCurrentPath(path)
    } catch {
      message.error('加载文件失败')
    } finally {
      setLoading(false)
    }
  }

  const handleNavigate = (path: string) => {
    loadFiles(path)
  }

  const handleNavigateParent = () => {
    const parentPath = currentPath.split('/').slice(0, -1).join('/')
    loadFiles(parentPath)
  }

  const handleDelete = async (path: string) => {
    try {
      await deleteFile(path)
      message.success('删除成功')
      loadFiles(currentPath)
    } catch {
      message.error('删除失败')
    }
  }

  const handleDownload = async (path: string) => {
    try {
      const blob = await downloadFile(path)
      const url = window.URL.createObjectURL(blob)
      const a = document.createElement('a')
      a.href = url
      a.download = path.split('/').pop() || 'file'
      document.body.appendChild(a)
      a.click()
      window.URL.revokeObjectURL(url)
      a.remove()
    } catch {
      message.error('下载失败')
    }
  }

  const uploadProps: UploadProps = {
    showUploadList: false,
    beforeUpload: async (file) => {
      const path = currentPath ? `${currentPath}/${file.name}` : file.name
      try {
        await uploadFile(path, file)
        message.success('上传成功')
        loadFiles(currentPath)
      } catch {
        message.error('上传失败')
      }
      return false
    },
  }

  const getBreadcrumbItems = () => {
    const items = [
      { title: <a onClick={() => handleNavigate('')}><HomeOutlined /></a> },
    ]
    
    if (currentPath) {
      const parts = currentPath.split('/')
      parts.forEach((part, index) => {
        const path = parts.slice(0, index + 1).join('/')
        items.push({
          title: <a onClick={() => handleNavigate(path)}>{part}</a>,
        })
      })
    }
    
    return items
  }

  const columns = [
    {
      title: '名称',
      dataIndex: 'name',
      key: 'name',
      render: (name: string, record: FileInfo) => (
        <Space>
          {record.is_dir ? <FolderOutlined style={{ color: '#faad14' }} /> : <FileOutlined />}
          {record.is_dir ? (
            <a onClick={() => handleNavigate(currentPath ? `${currentPath}/${name}` : name)}>
              {name}
            </a>
          ) : (
            <span>{name}</span>
          )}
        </Space>
      ),
    },
    ...(isMobile ? [] : [
      {
        title: '类型',
        dataIndex: 'is_dir',
        key: 'type',
        width: 80,
        render: (isDir: boolean) => (isDir ? '目录' : '文件'),
      },
      {
        title: '大小',
        dataIndex: 'size',
        key: 'size',
        width: 100,
        render: (size: number, record: FileInfo) => (record.is_dir ? '-' : formatSize(size)),
      },
      {
        title: '修改时间',
        dataIndex: 'mod_time',
        key: 'mod_time',
        width: 160,
        render: (time: string) => dayjs(time).format('YYYY-MM-DD HH:mm'),
      },
    ]),
    {
      title: '操作',
      key: 'action',
      width: isMobile ? 80 : 150,
      render: (_: unknown, record: FileInfo) => {
        const path = currentPath ? `${currentPath}/${record.name}` : record.name
        return (
          <Space size="small">
            {!record.is_dir && (
              <Button
                type="link"
                icon={<DownloadOutlined />}
                onClick={() => handleDownload(path)}
                size="small"
              >
                {!isMobile && '下载'}
              </Button>
            )}
            <Popconfirm
              title="确定要删除吗？"
              onConfirm={() => handleDelete(path)}
              okText="确定"
              cancelText="取消"
            >
              <Button type="link" danger icon={<DeleteOutlined />} size="small">
                {!isMobile && '删除'}
              </Button>
            </Popconfirm>
          </Space>
        )
      },
    },
  ]

  return (
    <div>
      <div 
        style={{ 
          marginBottom: 16, 
          display: 'flex', 
          flexDirection: isMobile ? 'column' : 'row',
          justifyContent: 'space-between', 
          alignItems: isMobile ? 'stretch' : 'center',
          gap: isMobile ? 12 : 0,
        }}
      >
        <Breadcrumb items={getBreadcrumbItems()} />
        <Upload {...uploadProps}>
          <Button icon={<UploadOutlined />} block={isMobile}>上传文件</Button>
        </Upload>
      </div>

      {currentPath && (
        <div style={{ marginBottom: 16 }}>
          <Button onClick={handleNavigateParent}>.. (返回上一级)</Button>
        </div>
      )}

      <Table
        columns={columns}
        dataSource={files}
        rowKey="name"
        loading={loading}
        locale={{
          emptyText: <Empty description="暂无文件" />,
        }}
        pagination={false}
        scroll={{ x: isMobile ? 300 : undefined }}
        size={isMobile ? 'small' : 'middle'}
      />
    </div>
  )
}
