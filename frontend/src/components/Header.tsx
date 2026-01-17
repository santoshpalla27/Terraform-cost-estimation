export function Header() {
    return (
        <header className="bg-white border-b border-slate-200 shadow-sm">
            <div className="max-w-7xl mx-auto px-4 py-4 flex items-center justify-between">
                <div className="flex items-center gap-3">
                    <div className="w-10 h-10 bg-gradient-to-br from-primary-500 to-primary-700 rounded-lg flex items-center justify-center">
                        <svg className="w-6 h-6 text-white" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 7h6m0 10v-3m-3 3h.01M9 17h.01M9 14h.01M12 14h.01M15 11h.01M12 11h.01M9 11h.01M7 21h10a2 2 0 002-2V5a2 2 0 00-2-2H7a2 2 0 00-2 2v14a2 2 0 002 2z" />
                        </svg>
                    </div>
                    <div>
                        <h1 className="text-xl font-bold text-slate-900">Terraform Cost</h1>
                        <p className="text-xs text-slate-500">Enterprise Cost Estimation</p>
                    </div>
                </div>

                <nav className="flex items-center gap-6">
                    <a href="#" className="text-sm font-medium text-primary-600 hover:text-primary-700">Dashboard</a>
                    <a href="#" className="text-sm font-medium text-slate-600 hover:text-slate-900">Diff</a>
                    <a href="#" className="text-sm font-medium text-slate-600 hover:text-slate-900">Policies</a>
                    <a href="#" className="text-sm font-medium text-slate-600 hover:text-slate-900">Settings</a>
                </nav>
            </div>
        </header>
    )
}
