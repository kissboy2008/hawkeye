import { useEffect, useCallback, useState, useRef } from 'react'
import { bgImages } from '../api/client'

const bgPresets = [
  {
    label: '预设1',
    url: '/preset_bg/bg1.jpg',
    thumb: '/preset_bg/bg1.jpg',
  },
  {
    label: '预设2',
    url: '/preset_bg/bg2.jpg',
    thumb: '/preset_bg/bg2.jpg',
  },
  {
    label: '预设3',
    url: '/preset_bg/bg3.jpg',
    thumb: '/preset_bg/bg3.jpg',
  },
  {
    label: '预设4',
    url: '/preset_bg/bg4.jpg',
    thumb: '/preset_bg/bg4.jpg',
  },
  {
    label: '预设5',
    url: '/preset_bg/bg5.jpg',
    thumb: '/preset_bg/bg5.jpg',
  },
]

interface CustomBgImage {
  filename: string
  url: string
}

interface BgSettingsPanelProps {
  open: boolean
  onClose: () => void
  selectedBg: number
  onSelectBg: (idx: number) => void
  selectedCustomUrl: string
  onSelectCustom: (url: string) => void
}

export default function BgSettingsPanel({
  open, onClose, selectedBg, onSelectBg,
  selectedCustomUrl, onSelectCustom,
}: BgSettingsPanelProps) {
  const [customImages, setCustomImages] = useState<CustomBgImage[]>([])
  const [uploading, setUploading] = useState(false)
  const [uploadError, setUploadError] = useState('')
  const fileInputRef = useRef<HTMLInputElement>(null)

  const handleEsc = useCallback((e: KeyboardEvent) => {
    if (e.key === 'Escape') onClose()
  }, [onClose])

  useEffect(() => {
    if (open) {
      document.addEventListener('keydown', handleEsc)
      loadCustomImages()
      return () => document.removeEventListener('keydown', handleEsc)
    }
  }, [open, handleEsc])

  const loadCustomImages = async () => {
    try {
      const data = await bgImages.list()
      setCustomImages(data.images || [])
    } catch {
      // ignore
    }
  }

  const handleUpload = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0]
    if (!file) return

    setUploadError('')
    setUploading(true)
    try {
      const result = await bgImages.upload(file)
      setCustomImages(prev => [...prev, { filename: result.filename, url: result.url }])
    } catch (err: any) {
      setUploadError(err.message || '上传失败')
    } finally {
      setUploading(false)
      if (fileInputRef.current) fileInputRef.current.value = ''
    }
  }

  const handleDeleteCustom = async (filename: string, e: React.MouseEvent) => {
    e.stopPropagation()
    try {
      await bgImages.delete(filename)
      setCustomImages(prev => prev.filter(img => img.filename !== filename))
      // 如果删除的是当前选中的自定义背景，清除选择
      if (selectedCustomUrl === `/custom_bg/${filename}`) {
        onSelectCustom('')
        onSelectBg(-1)
      }
    } catch {
      // ignore
    }
  }

  const handleClickCustom = (url: string) => {
    onSelectCustom(url)
    onSelectBg(-2) // -2 表示自定义背景
  }

  const isCustomSelected = selectedCustomUrl !== '' && selectedBg === -2
  const isNoneSelected = selectedBg === -1 && selectedCustomUrl === ''

  return (
    <>
      {/* Backdrop */}
      <div className={`bg-panel-backdrop ${open ? 'show' : ''}`} onClick={onClose} />

      {/* Panel */}
      <div className={`bg-panel ${open ? 'open' : ''}`}>
        <button className="bg-panel-close-btn" onClick={onClose} title="关闭">✕</button>

        {/* Top 1/3: Presets */}
        <div className="bg-presets-section">
          <div className="bg-section-label">🎨 选择背景</div>
          <div className="bg-preset-grid">
            <div
              className={`bg-preset-card none ${isNoneSelected ? 'active' : ''}`}
              onClick={() => { onSelectBg(-1); onSelectCustom('') }}
            >
              <span>🚫 无背景</span>
            </div>
            {bgPresets.map((bg, i) => (
              <div
                key={i}
                className={`bg-preset-card ${selectedBg === i && !isCustomSelected ? 'active' : ''}`}
                style={{ backgroundImage: `url('${bg.thumb}')` }}
                onClick={() => { onSelectBg(i); onSelectCustom('') }}
              >
                <span>{bg.label}</span>
              </div>
            ))}
          </div>
        </div>

        {/* Bottom 2/3: Custom upload */}
        <div className="bg-custom-section">
          <div className="bg-section-label">📁 自定义</div>

          {/* Upload area */}
          <div
            className="bg-upload-zone"
            onClick={() => fileInputRef.current?.click()}
          >
            <input
              ref={fileInputRef}
              type="file"
              accept=".jpg,.jpeg,.png,.webp,.gif"
              onChange={handleUpload}
              style={{ display: 'none' }}
            />
            {uploading ? (
              <span className="bg-upload-text">上传中...</span>
            ) : (
              <>
                <span className="bg-upload-icon">⬆</span>
                <span className="bg-upload-text">点击上传图片</span>
                <span className="bg-upload-hint">支持 JPG、PNG、WebP、GIF，最大 10MB</span>
              </>
            )}
          </div>

          {uploadError && (
            <div className="bg-upload-error">{uploadError}</div>
          )}

          {/* Custom image thumbnails */}
          {customImages.length > 0 && (
            <div className="bg-custom-grid">
              {customImages.map((img) => (
                <div
                  key={img.filename}
                  className={`bg-custom-thumb ${selectedCustomUrl === img.url ? 'active' : ''}`}
                  onClick={() => handleClickCustom(img.url)}
                >
                  <img src={img.url} alt={img.filename} />
                  <button
                    className="bg-custom-delete"
                    onClick={(e) => handleDeleteCustom(img.filename, e)}
                    title="删除"
                  >
                    ✕
                  </button>
                </div>
              ))}
            </div>
          )}

          {customImages.length === 0 && (
            <div className="bg-custom-empty">还没有上传自定义背景图片</div>
          )}
        </div>
      </div>
    </>
  )
}
