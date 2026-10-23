import React from 'react';

/** Page identity stays outside the fixed geometry of the desktop game windows. */
export default function ScreenHeading({ title, subtitle }: { title: string; subtitle: string }) {
  return <header className="md-screen-heading"><span aria-hidden="true" /><div><h1>{title}</h1><p>{subtitle}</p></div><small>GOYGOPRO / DUEL TERMINAL</small></header>;
}
