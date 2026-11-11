import { useEffect, useState, useRef } from 'react'
import { useOutletContext } from 'react-router-dom'
import { bgImages } from '../api/client'
import type { BgContext } from '../lib/bgContext'

const bgPresets = [
  { label: '预设1', url: '/preset_bg/bg1.jpg', thumb: '/preset_bg/bg1.jpg' },
  { label: '预设2', url: '/preset_bg/bg2.jpg', thumb: '/preset_bg/bg2.jpg' },
  { label: '预设3', url: '/preset_bg/bg3.jpg', thumb: '/preset_bg/bg3.jpg' },
  { label: '预设4', url: '/preset_bg/bg4.jpg', thumb: '/preset_bg/bg4.jpg' },
  { label: '预设5', url: '/preset_bg/bg5.jpg', thumb: '/preset_bg/bg5.jpg' },
]

interface CustomBgImage {
  filename: string
  url: string
}

export default function BgSettings() {
  const { bgIndex, bgCustomUrl, selectBg, selectCustom } = useOutletContext<BgContext>()
  const [customImages, setCustomImages] = useState<CustomBgImage[]>([])
  const [uploading, setUploading] = useState(false)
  const [uploadError, setUploadError] = useState('')
  const fileInputRef = useRef<HTMLInputElement>(null)

  useEffect(() => {
    loadCustomImages()
  }, [])

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
      if (bgCustomUrl === `/custom_bg/${filename}`) {
        selectCustom('')
        selectBg(-1)
      }
    } catch {
      // ignore
    }
  }

  const handleClickCustom = (url: string) => {
    selectCustom(url)
    selectBg(-2)
  }

  const isCustomSelected = bgCustomUrl !== '' && bgIndex === -2
  const isNoneSelected = bgIndex === -1 && bgCustomUrl === ''

  return (
    <div className="py-4 px-2 md:py-6 md:px-3 max-w-[1600px] mx-auto">
      {/* Header */}
      <div className="mb-4 md:mb-6">
        <h2 className="page-title">背景设置</h2>
        <p className="text-fg-3 text-sm">挑一张预设图或用你自己的图片，点击即时生效</p>
      </div>

      {/* 预设背景 */}
      <section className="bg-bg-card/70 rounded-xl p-4 md:p-5 mb-4">
        <div className="flex items-baseline justify-between mb-3">
          <h3 className="text-sm font-semibold text-fg-1">🎨 选择背景</h3>
          <span className="text-xs text-fg-4">当前：
            {isNoneSelected ? '无背景' : isCustomSelected ? '自定义图片' : bgIndex >= 0 ? bgPresets[bgIndex]?.label : '—'}
          </span>
        </div>
        <div className="grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-6 gap-3">
          <div
            className={`bg-preset-card none aspect-video ${isNoneSelected ? 'active' : ''}`}
            onClick={() => { selectBg(-1); selectCustom('') }}
          >
            <span>🚫 无背景</span>
          </div>
          {bgPresets.map((bg, i) => (
            <div
              key={i}
              className={`bg-preset-card aspect-video ${bgIndex === i && !isCustomSelected ? 'active' : ''}`}
              style={{ backgroundImage: `url('${bg.thumb}')` }}
              onClick={() => { selectBg(i); selectCustom('') }}
            >
              <span>{bg.label}</span>
            </div>
          ))}
        </div>
      </section>

      {/* 自定义背景 */}
      <section className="bg-bg-card/70 rounded-xl p-4 md:p-5">
        <h3 className="text-sm font-semibold text-fg-1 mb-3">📁 自定义</h3>

        <div className="max-w-xl">
          <div className="bg-upload-zone" onClick={() => fileInputRef.current?.click()}>
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

          {uploadError && <div className="bg-upload-error">{uploadError}</div>}
        </div>

        {customImages.length > 0 ? (
          <div className="grid grid-cols-2 sm:grid-cols-4 lg:grid-cols-6 gap-3 mt-4">
            {customImages.map((img) => (
              <div
                key={img.filename}
                className={`bg-custom-thumb ${bgCustomUrl === img.url ? 'active' : ''}`}
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
        ) : (
          <div className="bg-custom-empty">还没有上传自定义背景图片</div>
        )}
      </section>
    </div>
  )
}
