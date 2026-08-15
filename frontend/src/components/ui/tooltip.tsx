import * as React from 'react'

interface TooltipProps {
  content: string
  children: React.ReactNode
  side?: 'top' | 'bottom' | 'left' | 'right'
}

export function Tooltip({ content, children, side = 'bottom' }: TooltipProps) {
  const [visible, setVisible] = React.useState(false)
  const tooltipRef = React.useRef<HTMLDivElement>(null)

  const [actualSide, setActualSide] = React.useState(side)
  const [shownSide, setShownSide] = React.useState<{ visible: boolean; side: string }>({ visible: false, side })
  if (shownSide.visible !== visible || shownSide.side !== side) {
    setShownSide({ visible, side })
    if (visible) setActualSide(side)
  }

  React.useEffect(() => {
    if (!visible) return

    const checkOverflow = () => {
      const el = tooltipRef.current
      if (!el) return

      const rect = el.getBoundingClientRect()
      const padding = 4

      const overflowsTop = rect.top < padding
      const overflowsBottom = rect.bottom > window.innerHeight - padding
      const overflowsLeft = rect.left < padding
      const overflowsRight = rect.right > window.innerWidth - padding

      if (side === 'bottom' && overflowsBottom) {
        setActualSide('top')
      } else if (side === 'top' && overflowsTop) {
        setActualSide('bottom')
      } else if (side === 'right' && overflowsRight) {
        setActualSide('left')
      } else if (side === 'left' && overflowsLeft) {
        setActualSide('right')
      } else {
        setActualSide(side)
      }
    }

    requestAnimationFrame(checkOverflow)
  }, [visible, side])

  const positionClass = {
    top: 'bottom-full mb-1',
    bottom: 'top-full mt-1',
    left: 'right-full mr-1',
    right: 'left-full ml-1',
  }[actualSide]

  return (
    <div className="relative inline-flex" onMouseEnter={() => setVisible(true)} onMouseLeave={() => setVisible(false)}>
      {children}
      {visible && (
        <div
          ref={tooltipRef}
          className={`absolute ${positionClass} z-50 rounded-md bg-neutral-800 px-2 py-1 text-xs text-neutral-50 whitespace-nowrap`}
        >
          {content}
        </div>
      )}
    </div>
  )
}
