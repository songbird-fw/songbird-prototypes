import React from 'react';

const Widget = ({ title, icon: Icon, children, className = "", headerAction }) => {
  return (
    <div className={`bg-white rounded-lg border border-slate-200 shadow-sm overflow-hidden flex flex-col ${className}`}>
      <div className="px-3 py-2 border-b border-slate-100 bg-slate-50/50 flex items-center justify-between">
        <h3 className="text-[10px] font-bold text-slate-500 uppercase tracking-wider flex items-center gap-2">
          {Icon && <Icon size={12} />} {title}
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
