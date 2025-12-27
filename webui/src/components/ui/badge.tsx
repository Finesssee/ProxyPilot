import { Slot } from "@radix-ui/react-slot";
import { cva, type VariantProps } from "class-variance-authority";
import type * as React from "react";

import { cn } from "@/lib/utils";

const badgeVariants = cva(
	"inline-flex items-center justify-center rounded-sm border px-1.5 py-0.5 w-fit whitespace-nowrap shrink-0 [&>svg]:size-3 [&>svg]:pointer-events-none focus-visible:border-ring focus-visible:ring-ring/50 focus-visible:ring-[3px] aria-invalid:ring-destructive/20 dark:aria-invalid:ring-destructive/40 aria-invalid:border-destructive transition-[color,box-shadow] overflow-hidden font-mono text-[0.65rem] uppercase tracking-wide",
	{
		variants: {
			variant: {
				default:
					"border-transparent bg-[hsl(var(--accent-primary))] text-[hsl(var(--bg-primary))] font-semibold [a&]:hover:bg-[hsl(var(--accent-primary))]/90",
				secondary:
					"border-[hsl(var(--border-primary))] bg-[hsl(var(--bg-secondary))] text-[hsl(var(--text-secondary))] [a&]:hover:bg-[hsl(var(--bg-tertiary))]",
				destructive:
					"border-transparent bg-[hsl(var(--status-offline))] text-white [a&]:hover:bg-[hsl(var(--status-offline))]/90 focus-visible:ring-[hsl(var(--status-offline))]/20",
				outline:
					"border-[hsl(var(--border-primary))] bg-transparent text-[hsl(var(--text-secondary))] [a&]:hover:bg-[hsl(var(--bg-tertiary))] [a&]:hover:text-[hsl(var(--text-primary))]",
				success:
					"border-transparent bg-[hsl(var(--status-online))] text-white font-semibold [a&]:hover:bg-[hsl(var(--status-online))]/90",
				warning:
					"border-transparent bg-[hsl(var(--status-warning))] text-[hsl(var(--bg-primary))] font-semibold [a&]:hover:bg-[hsl(var(--status-warning))]/90",
				processing:
					"border-transparent bg-[hsl(var(--status-processing))] text-white font-semibold [a&]:hover:bg-[hsl(var(--status-processing))]/90",
			},
		},
		defaultVariants: {
			variant: "default",
		},
	},
);

function Badge({
	className,
	variant,
	asChild = false,
	...props
}: React.ComponentProps<"span"> &
	VariantProps<typeof badgeVariants> & { asChild?: boolean }) {
	const Comp = asChild ? Slot : "span";

	return (
		<Comp
			data-slot="badge"
			className={cn(badgeVariants({ variant }), className)}
			{...props}
		/>
	);
}

// Dot color mapping based on variant
const dotColorMap: Record<string, string> = {
	default: "bg-[hsl(var(--bg-primary))]",
	secondary: "bg-[hsl(var(--text-secondary))]",
	destructive: "bg-white",
	outline: "bg-current",
	success: "bg-white",
	warning: "bg-[hsl(var(--bg-primary))]",
	processing: "bg-white",
};

function BadgeWithDot({
	className,
	variant = "default",
	asChild = false,
	children,
	dotClassName,
	...props
}: React.ComponentProps<"span"> &
	VariantProps<typeof badgeVariants> & {
		asChild?: boolean;
		dotClassName?: string;
	}) {
	const Comp = asChild ? Slot : "span";
	const dotColor = dotColorMap[variant || "default"] || "bg-current";

	return (
		<Comp
			data-slot="badge"
			className={cn(
				badgeVariants({ variant }),
				"gap-1.5",
				className,
			)}
			{...props}
		>
			<span
				className={cn(
					"w-1.5 h-1.5 rounded-full shrink-0",
					dotColor,
					dotClassName,
				)}
				aria-hidden="true"
			/>
			{children}
		</Comp>
	);
}

export { Badge, BadgeWithDot, badgeVariants };
