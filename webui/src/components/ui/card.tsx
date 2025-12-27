import type * as React from "react";

import { cn } from "@/lib/utils";

/**
 * Industrial "PILOT COMMAND" Card Component
 * Sharp corners, solid backgrounds, layered panel depth with inset shadows
 */

function Card({ className, ...props }: React.ComponentProps<"div">) {
	return (
		<div
			data-slot="card"
			className={cn(
				// Industrial panel base
				"flex flex-col gap-6 rounded-md py-6",
				// Solid background with border
				"bg-[var(--bg-panel)] border border-[var(--border-subtle)]",
				// Text color
				"text-card-foreground",
				// Inset shadows for depth effect + offset shadow
				"shadow-[inset_0_1px_0_0_var(--border-subtle),inset_0_-1px_0_0_oklch(0_0_0/0.2),0_2px_0_0_oklch(0_0_0/0.1)]",
				className,
			)}
			{...props}
		/>
	);
}

/**
 * Interactive Card variant with hover effects
 * Use: <Card className="card-interactive" />
 */
function CardInteractive({
	className,
	...props
}: React.ComponentProps<"div">) {
	return (
		<div
			data-slot="card"
			className={cn(
				// Industrial panel base
				"flex flex-col gap-6 rounded-md py-6",
				// Solid background with border
				"bg-[var(--bg-panel)] border border-[var(--border-subtle)]",
				// Text color
				"text-card-foreground",
				// Inset shadows for depth effect + offset shadow
				"shadow-[inset_0_1px_0_0_var(--border-subtle),inset_0_-1px_0_0_oklch(0_0_0/0.2),0_2px_0_0_oklch(0_0_0/0.1)]",
				// Interactive hover state
				"transition-all duration-200 cursor-pointer",
				"hover:-translate-y-0.5 hover:border-[var(--border-accent)]",
				"hover:shadow-[0_0_0_1px_var(--accent-primary),0_0_20px_oklch(0.75_0.18_55/0.2)]",
				className,
			)}
			{...props}
		/>
	);
}

function CardHeader({ className, ...props }: React.ComponentProps<"div">) {
	return (
		<div
			data-slot="card-header"
			className={cn(
				// Grid layout for header content
				"@container/card-header grid auto-rows-min grid-rows-[auto_auto] items-start gap-2 px-6",
				"has-data-[slot=card-action]:grid-cols-[1fr_auto]",
				// Bottom border separator for industrial look
				"border-b border-[var(--border-subtle)] pb-4 mb-2",
				className,
			)}
			{...props}
		/>
	);
}

function CardTitle({ className, ...props }: React.ComponentProps<"div">) {
	return (
		<div
			data-slot="card-title"
			className={cn(
				// Display font (Space Grotesk) - larger and bolder
				"font-display text-lg font-bold leading-tight tracking-tight",
				className,
			)}
			{...props}
		/>
	);
}

function CardDescription({ className, ...props }: React.ComponentProps<"div">) {
	return (
		<div
			data-slot="card-description"
			className={cn(
				// Uppercase small text for industrial aesthetic
				"text-muted-foreground text-xs uppercase tracking-wider",
				className,
			)}
			{...props}
		/>
	);
}

/**
 * Technical Card Description variant with monospace font
 * Use for technical/code-related cards
 */
function CardDescriptionMono({
	className,
	...props
}: React.ComponentProps<"div">) {
	return (
		<div
			data-slot="card-description"
			className={cn(
				// Monospace font for technical cards
				"text-muted-foreground text-xs font-mono uppercase tracking-wider",
				className,
			)}
			{...props}
		/>
	);
}

function CardContent({ className, ...props }: React.ComponentProps<"div">) {
	return (
		<div
			data-slot="card-content"
			className={cn("px-6", className)}
			{...props}
		/>
	);
}

function CardFooter({ className, ...props }: React.ComponentProps<"div">) {
	return (
		<div
			data-slot="card-footer"
			className={cn(
				"flex items-center px-6 pt-4",
				// Top border separator
				"border-t border-[var(--border-subtle)]",
				className,
			)}
			{...props}
		/>
	);
}

export {
	Card,
	CardInteractive,
	CardHeader,
	CardTitle,
	CardDescription,
	CardDescriptionMono,
	CardContent,
	CardFooter,
};
