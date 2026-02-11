import { NgModule } from '@angular/core';
import { BrowserModule } from '@angular/platform-browser';

import { BrowserAnimationsModule } from '@angular/platform-browser/animations';
import { HttpClientModule } from '@angular/common/http';

import { AppRoutingModule } from './app-routing.module';
import { AppComponent } from './app.component';
import { JobsComponent } from './jobs/jobs.component';
import { DashboardComponent } from './dashboard/dashboard.component';
import { SharedModule } from './shared.module';
import { NodesComponent } from './nodes/nodes.component';
import { NodeCardComponent } from './nodes/node-card/node-card.component';
import { GpuSlotsComponent } from './nodes/gpu-slots/gpu-slots.component';
import { NodeGridComponent } from './nodes/node-grid/node-grid.component';

@NgModule({
  declarations: [
    AppComponent,
    DashboardComponent,
    JobsComponent,
    NodesComponent,
    NodeCardComponent,
    GpuSlotsComponent,
    NodeGridComponent
  ],
  imports: [
    BrowserModule,
    BrowserAnimationsModule,
    HttpClientModule,
    AppRoutingModule,
    SharedModule
  ],
  providers: [],
  bootstrap: [AppComponent]
})
export class AppModule { }
