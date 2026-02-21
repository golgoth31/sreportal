import { Routes } from '@angular/router';

export const routes: Routes = [
  { path: '', redirectTo: '/main/links', pathMatch: 'full' },
  {
    path: ':portalName/links',
    loadComponent: () =>
      import('./pages/links/links.component').then(m => m.LinksComponent),
  },
];
