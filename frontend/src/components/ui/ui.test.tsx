import { describe, it, expect, vi } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'
import { Button } from './button'
import { Input } from './input'
import { Tooltip } from './tooltip'
import { Separator } from './separator'

describe('Button', () => {
  it('renders with default variant and size', () => {
    render(<Button>Click me</Button>)
    const button = screen.getByRole('button', { name: 'Click me' })
    expect(button).toBeInTheDocument()
    expect(button).toHaveClass('bg-neutral-900') // default variant
    expect(button).toHaveClass('h-9') // default size
  })

  it('renders ghost variant', () => {
    render(<Button variant="ghost">Ghost</Button>)
    const button = screen.getByRole('button', { name: 'Ghost' })
    expect(button).toHaveClass('hover:bg-neutral-100')
  })

  it('renders outline variant', () => {
    render(<Button variant="outline">Outline</Button>)
    const button = screen.getByRole('button', { name: 'Outline' })
    expect(button).toHaveClass('border')
  })

  it('renders icon size', () => {
    render(<Button size="icon" aria-label="icon-btn">×</Button>)
    const button = screen.getByRole('button', { name: 'icon-btn' })
    expect(button).toHaveClass('h-9', 'w-9')
  })

  it('renders sm size', () => {
    render(<Button size="sm">Small</Button>)
    const button = screen.getByRole('button', { name: 'Small' })
    expect(button).toHaveClass('h-8')
  })

  it('renders lg size', () => {
    render(<Button size="lg">Large</Button>)
    const button = screen.getByRole('button', { name: 'Large' })
    expect(button).toHaveClass('h-10')
  })

  it('is disabled when disabled prop is set', () => {
    render(<Button disabled>Disabled</Button>)
    const button = screen.getByRole('button', { name: 'Disabled' })
    expect(button).toBeDisabled()
    expect(button).toHaveClass('disabled:opacity-50')
  })

  it('calls onClick when clicked', () => {
    const onClick = vi.fn()
    render(<Button onClick={onClick}>Click</Button>)
    fireEvent.click(screen.getByRole('button', { name: 'Click' }))
    expect(onClick).toHaveBeenCalledTimes(1)
  })

  it('forwards additional props', () => {
    render(<Button data-testid="custom-btn">Custom</Button>)
    expect(screen.getByTestId('custom-btn')).toBeInTheDocument()
  })

  it('merges custom className', () => {
    render(<Button className="extra-class">Merged</Button>)
    const button = screen.getByRole('button', { name: 'Merged' })
    expect(button).toHaveClass('extra-class')
  })
})

describe('Input', () => {
  it('renders an input element', () => {
    render(<Input />)
    expect(screen.getByRole('textbox')).toBeInTheDocument()
  })

  it('accepts placeholder', () => {
    render(<Input placeholder="Type here" />)
    expect(screen.getByPlaceholderText('Type here')).toBeInTheDocument()
  })

  it('is disabled when disabled prop is set', () => {
    render(<Input disabled />)
    expect(screen.getByRole('textbox')).toBeDisabled()
  })

  it('forwards ref', () => {
    const refCallback = vi.fn()
    render(<Input ref={refCallback} />)
    expect(refCallback).toHaveBeenCalled()
  })

  it('handles onChange', () => {
    const onChange = vi.fn()
    render(<Input onChange={onChange} />)
    const input = screen.getByRole('textbox')
    fireEvent.change(input, { target: { value: 'test' } })
    expect(onChange).toHaveBeenCalledTimes(1)
  })

  it('merges custom className', () => {
    render(<Input className="custom-input" />)
    const input = screen.getByRole('textbox')
    expect(input).toHaveClass('custom-input')
  })

  it('has correct base styles', () => {
    render(<Input />)
    const input = screen.getByRole('textbox')
    expect(input).toHaveClass('rounded-md', 'border', 'border-neutral-200')
  })
})

describe('Tooltip', () => {
  it('renders children', () => {
    render(<Tooltip content="Tooltip text"><button>Hover me</button></Tooltip>)
    expect(screen.getByRole('button', { name: 'Hover me' })).toBeInTheDocument()
  })

  it('shows tooltip on mouse enter', () => {
    render(<Tooltip content="Tooltip text"><button>Hover me</button></Tooltip>)
    const wrapper = screen.getByRole('button', { name: 'Hover me' }).parentElement!
    fireEvent.mouseEnter(wrapper)
    expect(screen.getByText('Tooltip text')).toBeInTheDocument()
  })

  it('hides tooltip on mouse leave', () => {
    render(<Tooltip content="Tooltip text"><button>Hover me</button></Tooltip>)
    const wrapper = screen.getByRole('button', { name: 'Hover me' }).parentElement!
    fireEvent.mouseEnter(wrapper)
    expect(screen.getByText('Tooltip text')).toBeInTheDocument()
    fireEvent.mouseLeave(wrapper)
    expect(screen.queryByText('Tooltip text')).not.toBeInTheDocument()
  })

  it('positions tooltip at bottom by default', () => {
    render(<Tooltip content="Bottom tip"><button>Hover</button></Tooltip>)
    const wrapper = screen.getByRole('button', { name: 'Hover' }).parentElement!
    fireEvent.mouseEnter(wrapper)
    const tooltip = screen.getByText('Bottom tip')
    expect(tooltip).toHaveClass('top-full')
  })

  it('positions tooltip at top when side is top', () => {
    render(
      <Tooltip content="Top tip" side="top">
        <button>Hover</button>
      </Tooltip>
    )
    const wrapper = screen.getByRole('button', { name: 'Hover' }).parentElement!
    fireEvent.mouseEnter(wrapper)
    const tooltip = screen.getByText('Top tip')
    expect(tooltip).toHaveClass('bottom-full')
  })

  it('positions tooltip at left when side is left', () => {
    render(
      <Tooltip content="Left tip" side="left">
        <button>Hover</button>
      </Tooltip>
    )
    const wrapper = screen.getByRole('button', { name: 'Hover' }).parentElement!
    fireEvent.mouseEnter(wrapper)
    const tooltip = screen.getByText('Left tip')
    expect(tooltip).toHaveClass('right-full')
  })

  it('positions tooltip at right when side is right', () => {
    render(
      <Tooltip content="Right tip" side="right">
        <button>Hover</button>
      </Tooltip>
    )
    const wrapper = screen.getByRole('button', { name: 'Hover' }).parentElement!
    fireEvent.mouseEnter(wrapper)
    const tooltip = screen.getByText('Right tip')
    expect(tooltip).toHaveClass('left-full')
  })
})

describe('Separator', () => {
  it('renders a horizontal separator by default', () => {
    const { container } = render(<Separator />)
    const separator = container.firstChild as HTMLElement
    expect(separator).toHaveClass('h-px', 'w-full')
  })

  it('renders a vertical separator when orientation is vertical', () => {
    const { container } = render(<Separator orientation="vertical" />)
    const separator = container.firstChild as HTMLElement
    expect(separator).toHaveClass('h-full', 'w-px')
  })

  it('has role none when decorative is true', () => {
    const { container } = render(<Separator decorative={true} />)
    const separator = container.firstChild as HTMLElement
    expect(separator).toHaveAttribute('role', 'none')
  })

  it('has role separator when decorative is false', () => {
    const { container } = render(<Separator decorative={false} />)
    const separator = container.firstChild as HTMLElement
    expect(separator).toHaveAttribute('role', 'separator')
  })

  it('sets aria-orientation for vertical separator', () => {
    const { container } = render(<Separator orientation="vertical" />)
    const separator = container.firstChild as HTMLElement
    expect(separator).toHaveAttribute('aria-orientation', 'vertical')
  })

  it('does not set aria-orientation for horizontal separator', () => {
    const { container } = render(<Separator orientation="horizontal" />)
    const separator = container.firstChild as HTMLElement
    expect(separator).not.toHaveAttribute('aria-orientation')
  })

  it('merges custom className', () => {
    const { container } = render(<Separator className="my-sep" />)
    const separator = container.firstChild as HTMLElement
    expect(separator).toHaveClass('my-sep')
  })

  it('has neutral background color', () => {
    const { container } = render(<Separator />)
    const separator = container.firstChild as HTMLElement
    expect(separator).toHaveClass('bg-neutral-200')
  })
})
