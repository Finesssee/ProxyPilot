import * as React from "react"
import { cva } from "class-variance-authority"
import { cn } from "@/lib/utils"

// Status color definitions using oklch
const statusColors = {
  online: "oklch(0.72 0.19 145)",
  offline: "oklch(0.55 0.12 30)",
  warning: "oklch(0.80 0.16 80)",
  processing: "oklch(0.65 0.15 220)",
} as const

// Size definitions in pixels
const dotSizes = {
  sm: 8,
  md: 12,
  lg: 16,
} as const

type Status = "online" | "offline" | "warning" | "processing"
type Size = "sm" | "md" | "lg"

interface StatusIndicatorProps {
  status: Status
  size?: Size
  pulse?: boolean
  label?: string
  className?: string
}

const statusIndicatorVariants = cva(
  "inline-flex items-center gap-2",
  {
    variants: {
      size: {
        sm: "text-xs",
        md: "text-sm",
        lg: "text-base",
      },
    },
    defaultVariants: {
      size: "md",
    },
  }
)

const StatusIndicator = React.forwardRef<
  HTMLDivElement,
  StatusIndicatorProps & React.HTMLAttributes<HTMLDivElement>
>(({ status, size = "md", pulse, label, className, ...props }, ref) => {
  // Default pulse behavior: on for 'online' and 'processing'
  const shouldPulse = pulse ?? (status === "online" || status === "processing")
  const dotSize = dotSizes[size]
  const color = statusColors[status]

  return (
    <div
      ref={ref}
      className={cn(statusIndicatorVariants({ size }), className)}
      {...props}
    >
      <span
        className="relative inline-block rounded-full shrink-0"
        style={{
          width: dotSize,
          height: dotSize,
          backgroundColor: color,
          boxShadow: `0 0 ${dotSize / 2}px ${color}`,
          animation: shouldPulse ? "statusPulse 2s ease-in-out infinite" : undefined,
          // Use CSS custom property for the animation color
          ["--pulse-color" as string]: color,
        }}
        aria-hidden="true"
      />
      {label && (
        <span className="text-foreground">{label}</span>
      )}
      <style>{`
        @keyframes statusPulse {
          0%, 100% {
            box-shadow: 0 0 0 0 var(--pulse-color);
          }
          50% {
            box-shadow: 0 0 0 4px color-mix(in oklch, var(--pulse-color) 30%, transparent);
          }
        }
      `}</style>
    </div>
  )
})
StatusIndicator.displayName = "StatusIndicator"

// Badge variant styles
const statusBadgeVariants = cva(
  "inline-flex items-center gap-1.5 rounded-full border font-medium",
  {
    variants: {
      size: {
        sm: "px-2 py-0.5 text-xs",
        md: "px-2.5 py-1 text-sm",
        lg: "px-3 py-1.5 text-base",
      },
    },
    defaultVariants: {
      size: "md",
    },
  }
)

interface StatusBadgeProps {
  status: Status
  size?: Size
  pulse?: boolean
  label: string
  className?: string
}

const StatusBadge = React.forwardRef<
  HTMLDivElement,
  StatusBadgeProps & React.HTMLAttributes<HTMLDivElement>
>(({ status, size = "md", pulse, label, className, ...props }, ref) => {
  // Default pulse behavior: on for 'online' and 'processing'
  const shouldPulse = pulse ?? (status === "online" || status === "processing")
  const color = statusColors[status]

  // Smaller dot sizes for badge variant
  const badgeDotSizes = {
    sm: 6,
    md: 8,
    lg: 10,
  }
  const dotSize = badgeDotSizes[size]

  return (
    <div
      ref={ref}
      className={cn(
        statusBadgeVariants({ size }),
        "bg-background/50 backdrop-blur-sm",
        className
      )}
      style={{
        borderColor: `color-mix(in oklch, ${color} 40%, transparent)`,
        backgroundColor: `color-mix(in oklch, ${color} 10%, transparent)`,
      }}
      {...props}
    >
      <span
        className="relative inline-block rounded-full shrink-0"
        style={{
          width: dotSize,
          height: dotSize,
          backgroundColor: color,
          boxShadow: `0 0 ${dotSize / 2}px ${color}`,
          animation: shouldPulse ? "statusPulse 2s ease-in-out infinite" : undefined,
          ["--pulse-color" as string]: color,
        }}
        aria-hidden="true"
      />
      <span
        style={{
          color: color,
        }}
      >
        {label}
      </span>
      <style>{`
        @keyframes statusPulse {
          0%, 100% {
            box-shadow: 0 0 0 0 var(--pulse-color);
          }
          50% {
            box-shadow: 0 0 0 4px color-mix(in oklch, var(--pulse-color) 30%, transparent);
          }
        }
      `}</style>
    </div>
  )
})
StatusBadge.displayName = "StatusBadge"

export { StatusIndicator, StatusBadge, statusColors, type StatusIndicatorProps, type StatusBadgeProps }
