export function DragHandle({ listeners }: { listeners?: Record<string, unknown> | null }) {
  return (
    <button
      {...listeners}
      onClick={(e) => e.stopPropagation()}
      className="cursor-grab active:cursor-grabbing text-gray-600 hover:text-gray-400 px-1 py-1 opacity-0 group-hover:opacity-100 transition-opacity flex-shrink-0"
      title="拖拽排序"
    >
      <svg width="12" height="12" viewBox="0 0 12 12" fill="currentColor">
        <circle cx="3" cy="2" r="1.2" /><circle cx="9" cy="2" r="1.2" />
        <circle cx="3" cy="6" r="1.2" /><circle cx="9" cy="6" r="1.2" />
        <circle cx="3" cy="10" r="1.2" /><circle cx="9" cy="10" r="1.2" />
      </svg>
    </button>
  )
}
