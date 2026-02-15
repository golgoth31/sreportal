import { Routes } from '@angular/router';
import { LinksComponent } from './pages/links/links.component';

export const routes: Routes = [
  { path: '', redirectTo: '/main/links', pathMatch: 'full' },
  { path: ':portalName/links', component: LinksComponent },
];
