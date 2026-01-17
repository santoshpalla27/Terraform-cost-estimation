interface Props {
    confidence: number
    size?: 'sm' | 'md' | 'lg'
}

export function ConfidenceBadge({ confidence, size = 'md' }: Props) {
    const pct = confidence * 100

    let color = 'bg-green-100 text-green-800 border-green-200'
    let label = 'High'

    if (pct < 60) {
        color = 'bg-red-100 text-red-800 border-red-200'
        label = 'Low'
    } else if (pct < 80) {
        color = 'bg-yellow-100 text-yellow-800 border-yellow-200'
        label = 'Medium'
    }

    const sizeClasses = {
        sm: 'text-xs px-1.5 py-0.5',
        md: 'text-sm px-2 py-1',
        lg: 'text-base px-3 py-1.5',
    }

    return (
        <span className={`inline-flex items-center gap-1.5 rounded-full border ${color} ${sizeClasses[size]}`}>
            <span className="font-medium">{pct.toFixed(0)}%</span>
            <span className="text-opacity-75">{label}</span>
        </span>
    )
}
