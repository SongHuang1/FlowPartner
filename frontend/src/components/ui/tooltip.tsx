import * as React from 'react'

interface TooltipProps {
  content: string
  children: React.ReactNode
  side?: 'top' | 'bottom' | 'left' | 'right'
}

export function Tooltip({ content, children, side = 'bottom' }: TooltipProps) {
  const [visible, setVisible] = React.useState(false)

  const positionClass = {
    top: 'bottom-full mb-1',
    bottom: 'top-full mt-1',
    left: 'right-full mr-1',
    right: 'left-full ml-1',
  }[side]

  return (
    <div className="relative inline-flex" onMouseEnter={() => setVisible(true)} onMouseLeave={() => setVisible(false)}>
      {children}
      {visible && (
        <div className={`absolute ${positionClass} z-50 rounded-md bg-neutral-800 px-2 py-1 text-xs text-neutral-50 whitespace-nowrap`}>
          {content}
        </div>
      )}
    </div>
  )
}
