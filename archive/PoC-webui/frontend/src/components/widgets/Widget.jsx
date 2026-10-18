import React from 'react';

const Widget = ({ title, icon: Icon, children, className = "", headerAction }) => {
  return (
    <div className={`bg-white rounded-lg border border-slate-200 shadow-sm overflow-hidden flex flex-col h-full ${className}`}>
      <div className="px-2 py-1.5 border-b border-slate-100 bg-slate-50/50 flex items-center justify-between">
        <h3 className="text-[9px] font-black text-slate-500 uppercase tracking-widest flex items-center gap-1.5">
          {Icon && <Icon size={10} />} {title}
        </h3>
        {headerAction}
      </div>
      <div className="flex-1 overflow-hidden">
        {children}
      </div>
    </div>
  );
};

export default Widget;
