import subprocess
import time
import os
import signal
import sys
import atexit
from threading import Thread, Lock
from datetime import datetime
import queue
import psutil

# Kivy imports
from kivy.app import App
from kivy.uix.boxlayout import BoxLayout
from kivy.uix.gridlayout import GridLayout
from kivy.uix.label import Label
from kivy.uix.button import Button
from kivy.uix.scrollview import ScrollView
from kivy.uix.popup import Popup
from kivy.uix.textinput import TextInput
from kivy.clock import Clock
from kivy.uix.widget import Widget
from kivy.graphics import Color, Rectangle
from kivy.core.window import Window
from kivy.resources import resource_find

# Configurar el tema oscuro
Window.clearcolor = (0.1, 0.1, 0.1, 1)

def get_monospace_font():
    """Obtener una fuente monoespaciada disponible en el sistema"""
    fonts_to_try = [
        'Consolas',        # Windows
        'Courier New',     # Windows/Cross-platform
        'Monaco',          # macOS
        'DejaVu Sans Mono', # Linux
        'Liberation Mono', # Linux
        'Menlo',          # macOS
        'Lucida Console'  # Windows
    ]
    
    # En Windows, Kivy puede usar fuentes del sistema directamente
    if os.name == 'nt':
        return 'Consolas'  # Fuente monoespaciada nativa de Windows
    else:
        return 'Liberation Mono'  # Fuente común en Linux

class LogViewerPopup(Popup):
    def __init__(self, instance_info, **kwargs):
        super().__init__(**kwargs)
        self.instance_info = instance_info
        self.title = f"Logs - {instance_info['type']} Puerto {instance_info['port']}"
        self.size_hint = (0.9, 0.9)
        
        # Layout principal
        content = BoxLayout(orientation='vertical', padding=10, spacing=10)
        
        # Área de logs con manejo seguro de fuentes
        try:
            font_name = get_monospace_font()
        except Exception:
            font_name = None  # Usar fuente predeterminada
            
        self.log_display = TextInput(
            text="Cargando logs...",
            multiline=True,
            readonly=True,
            font_size='11sp',
            background_color=(0.05, 0.05, 0.05, 1),
            foreground_color=(0.9, 0.9, 0.9, 1)
        )
        
        # Aplicar fuente solo si está disponible
        if font_name:
            try:
                self.log_display.font_name = font_name
            except Exception:
                pass  # Usar fuente predeterminada si falla
        
        scroll = ScrollView()
        scroll.add_widget(self.log_display)
        content.add_widget(scroll)
        
        # Botones
        button_layout = BoxLayout(size_hint_y=None, height=50, spacing=10)
        
        refresh_btn = Button(
            text="Actualizar",
            size_hint_x=0.3,
            background_color=(0.2, 0.6, 0.2, 1)
        )
        refresh_btn.bind(on_press=self.refresh_logs)
        
        close_btn = Button(
            text="Cerrar",
            size_hint_x=0.3,
            background_color=(0.6, 0.2, 0.2, 1)
        )
        close_btn.bind(on_press=self.dismiss)
        
        button_layout.add_widget(refresh_btn)
        button_layout.add_widget(Widget())  # Spacer
        button_layout.add_widget(close_btn)
        
        content.add_widget(button_layout)
        self.content = content
        
        # Cargar logs iniciales
        self.refresh_logs()
        
        # Auto-refresh cada 5 segundos
        self.refresh_event = Clock.schedule_interval(lambda dt: self.refresh_logs(), 5)
    
    def refresh_logs(self, *args):
        """Actualizar los logs de la instancia"""
        try:
            process = self.instance_info['process']
            if process and process.poll() is None:
                # Leer stdout y stderr de forma segura
                stdout_data = ""
                stderr_data = ""
                
                try:
                    # Verificar si hay datos disponibles para leer
                    if process.stdout and hasattr(process.stdout, 'readable') and process.stdout.readable():
                        stdout_data = process.stdout.read().decode('utf-8', errors='ignore')
                    elif hasattr(process, 'stdout') and process.stdout:
                        try:
                            stdout_data = process.stdout.read().decode('utf-8', errors='ignore')
                        except Exception:
                            stdout_data = "Logs no disponibles (proceso activo)"
                except Exception as e:
                    stdout_data = f"Error leyendo stdout: {str(e)}"
                
                try:
                    if process.stderr and hasattr(process.stderr, 'readable') and process.stderr.readable():
                        stderr_data = process.stderr.read().decode('utf-8', errors='ignore')
                    elif hasattr(process, 'stderr') and process.stderr:
                        try:
                            stderr_data = process.stderr.read().decode('utf-8', errors='ignore')
                        except Exception:
                            stderr_data = "Error logs no disponibles (proceso activo)"
                except Exception as e:
                    stderr_data = f"Error leyendo stderr: {str(e)}"
                
                # Información básica del proceso
                basic_info = f"Puerto: {self.instance_info['port']}\n"
                basic_info += f"Tipo: {self.instance_info['type']}\n"
                basic_info += f"Directorio: {self.instance_info['datadir']}\n"
                basic_info += f"PID: {process.pid if process else 'N/A'}\n"
                basic_info += f"Estado: {'Activo' if process and process.poll() is None else 'Inactivo'}\n"
                basic_info += "=" * 50 + "\n\n"
                
                # Combinar logs
                logs = basic_info
                if stdout_data.strip():
                    logs += f"=== STDOUT ===\n{stdout_data}\n\n"
                if stderr_data.strip():
                    logs += f"=== STDERR ===\n{stderr_data}\n\n"
                
                if not stdout_data.strip() and not stderr_data.strip():
                    logs += f"Proceso activo en puerto {self.instance_info['port']}\n"
                    logs += "Los logs aparecerán cuando la aplicación genere salida...\n"
                    logs += "Nota: En Windows, los logs pueden no estar disponibles inmediatamente."
                
            else:
                logs = f"❌ Proceso en puerto {self.instance_info['port']} no está activo\n"
                logs += f"Estado del proceso: {process.poll() if process else 'No existe'}"
            
            self.log_display.text = logs
            
        except Exception as e:
            error_msg = f"Error obteniendo logs: {str(e)}\n\n"
            error_msg += f"Puerto: {self.instance_info.get('port', 'N/A')}\n"
            error_msg += f"Tipo: {self.instance_info.get('type', 'N/A')}\n"
            error_msg += f"Directorio: {self.instance_info.get('datadir', 'N/A')}\n"
            self.log_display.text = error_msg
    
    def dismiss(self, *args):
        """Cerrar popup y limpiar eventos"""
        if hasattr(self, 'refresh_event'):
            self.refresh_event.cancel()
        super().dismiss()

class InstanceWidget(BoxLayout):
    def __init__(self, instance_info, **kwargs):
        super().__init__(**kwargs)
        self.instance_info = instance_info
        self.orientation = 'horizontal'
        self.size_hint_y = None
        self.height = 60
        self.padding = 5
        self.spacing = 10
        
        # Background color
        with self.canvas.before:
            Color(0.15, 0.15, 0.15, 1)
            self.rect = Rectangle(pos=self.pos, size=self.size)
        self.bind(pos=self.update_rect, size=self.update_rect)
        
        # Info de la instancia
        info_layout = BoxLayout(orientation='vertical', size_hint_x=0.7)
        
        # Tipo y puerto
        self.title_label = Label(
            text=f"{instance_info['type']} - Puerto {instance_info['port']}",
            font_size='14sp',
            bold=True,
            color=(1, 1, 1, 1),
            text_size=(None, None),
            halign='left'
        )
        
        # Estado
        self.status_label = Label(
            text="Verificando...",
            font_size='12sp',
            color=(0.8, 0.8, 0.8, 1),
            text_size=(None, None),
            halign='left'
        )
        
        info_layout.add_widget(self.title_label)
        info_layout.add_widget(self.status_label)
        
        # Estado visual
        self.status_indicator = Widget(size_hint_x=None, width=20)
        with self.status_indicator.canvas:
            Color(0.5, 0.5, 0.5, 1)
            self.status_circle = Rectangle(pos=(0, 0), size=(15, 15))
        
        # Botón de logs
        self.logs_btn = Button(
            text="Ver Logs",
            size_hint_x=None,
            width=100,
            background_color=(0.3, 0.5, 0.8, 1)
        )
        self.logs_btn.bind(on_press=self.show_logs)
        
        self.add_widget(info_layout)
        self.add_widget(self.status_indicator)
        self.add_widget(self.logs_btn)
        
        self.update_status()
    
    def update_rect(self, *args):
        self.rect.pos = self.pos
        self.rect.size = self.size
    
    def update_status(self):
        """Actualizar el estado de la instancia"""
        try:
            process = self.instance_info['process']
            if process and process.poll() is None:
                # Verificar si el puerto está en uso
                port = self.instance_info['port']
                port_active = self.is_port_in_use(port)
                
                if port_active:
                    status = "🟢 ACTIVO"
                    color = (0.2, 0.8, 0.2, 1)  # Verde
                else:
                    status = "🟡 INICIANDO"
                    color = (0.8, 0.8, 0.2, 1)  # Amarillo
                
                self.logs_btn.disabled = False
            else:
                status = "🔴 INACTIVO"
                color = (0.8, 0.2, 0.2, 1)  # Rojo
                self.logs_btn.disabled = True
            
            # Actualizar labels
            self.status_label.text = f"Estado: {status} | Directorio: {self.instance_info['datadir']}"
            
            # Actualizar indicador visual
            with self.status_indicator.canvas:
                self.status_indicator.canvas.clear()
                Color(*color)
                self.status_circle = Rectangle(
                    pos=(self.status_indicator.x + 2, self.status_indicator.center_y - 7),
                    size=(15, 15)
                )
                
        except Exception as e:
            self.status_label.text = f"Error: {str(e)}"
    
    def is_port_in_use(self, port):
        """Verificar si un puerto está en uso"""
        try:
            for conn in psutil.net_connections():
                if conn.laddr.port == port and conn.status == 'LISTEN':
                    return True
            return False
        except:
            return False
    
    def show_logs(self, *args):
        """Mostrar ventana de logs"""
        popup = LogViewerPopup(self.instance_info)
        popup.open()

class GoClusterMonitorApp(App):
    def __init__(self, **kwargs):
        super().__init__(**kwargs)
        self.manager = None
        self.monitor_thread = None
        self.running = True
        
    def build(self):
        self.title = "Monitor de Cluster Go"
        
        # Layout principal
        main_layout = BoxLayout(orientation='vertical', padding=10, spacing=10)
        
        # Header
        header = BoxLayout(size_hint_y=None, height=80, spacing=10)
        
        title_label = Label(
            text="🎯 Monitor de Cluster Go",
            font_size='20sp',
            bold=True,
            color=(1, 1, 1, 1)
        )
        
        # Botones de control
        control_layout = BoxLayout(orientation='vertical', size_hint_x=None, width=150)
        
        self.start_btn = Button(
            text="Iniciar Cluster",
            size_hint_y=None,
            height=35,
            background_color=(0.2, 0.6, 0.2, 1)
        )
        self.start_btn.bind(on_press=self.start_cluster)
        
        self.stop_btn = Button(
            text="Detener Cluster",
            size_hint_y=None,
            height=35,
            background_color=(0.6, 0.2, 0.2, 1),
            disabled=True
        )
        self.stop_btn.bind(on_press=self.stop_cluster)
        
        control_layout.add_widget(self.start_btn)
        control_layout.add_widget(self.stop_btn)
        
        header.add_widget(title_label)
        header.add_widget(control_layout)
        
        # Estado general
        self.status_label = Label(
            text="🔴 Cluster detenido",
            size_hint_y=None,
            height=30,
            font_size='16sp',
            color=(0.8, 0.8, 0.8, 1)
        )
        
        # Lista de instancias
        self.instances_layout = BoxLayout(orientation='vertical', spacing=5)
        
        scroll = ScrollView()
        scroll.add_widget(self.instances_layout)
        
        main_layout.add_widget(header)
        main_layout.add_widget(self.status_label)
        main_layout.add_widget(scroll)
        
        # Iniciar monitoreo
        Clock.schedule_interval(self.update_interface, 60)  # Cada 1 minuto
        Clock.schedule_interval(self.quick_update, 10)      # Cada 10 segundos para updates rápidos
        
        return main_layout
    
    def start_cluster(self, *args):
        """Iniciar el cluster"""
        try:
            self.start_btn.disabled = True
            self.status_label.text = "🟡 Iniciando cluster..."
            
            # Crear y iniciar el manager
            self.manager = GoAppManager()
            
            # Ejecutar en hilo separado
            self.monitor_thread = Thread(target=self.run_cluster)
            self.monitor_thread.daemon = True
            self.monitor_thread.start()
            
        except Exception as e:
            self.status_label.text = f"❌ Error iniciando: {str(e)}"
            self.start_btn.disabled = False
    
    def run_cluster(self):
        """Ejecutar el cluster en hilo separado"""
        try:
            if self.manager.start_all_instances():
                Clock.schedule_once(lambda dt: self.on_cluster_started(), 0)
            else:
                Clock.schedule_once(lambda dt: self.on_cluster_error(), 0)
        except Exception as e:
            Clock.schedule_once(lambda dt: self.on_cluster_error(str(e)), 0)
    
    def on_cluster_started(self):
        """Callback cuando el cluster inicia correctamente"""
        self.status_label.text = f"🟢 Cluster activo - {len(self.manager.processes)} instancias"
        self.stop_btn.disabled = False
        self.update_instances_display()
    
    def on_cluster_error(self, error_msg="Error desconocido"):
        """Callback cuando hay error al iniciar"""
        self.status_label.text = f"❌ Error: {error_msg}"
        self.start_btn.disabled = False
    
    def stop_cluster(self, *args):
        """Detener el cluster"""
        try:
            self.stop_btn.disabled = True
            self.status_label.text = "🟡 Deteniendo cluster..."
            
            if self.manager:
                self.manager.stop_all_instances()
                
            self.status_label.text = "🔴 Cluster detenido"
            self.start_btn.disabled = False
            
            # Limpiar la lista de instancias
            self.instances_layout.clear_widgets()
            
        except Exception as e:
            self.status_label.text = f"❌ Error deteniendo: {str(e)}"
    
    def update_instances_display(self):
        """Actualizar la visualización de instancias"""
        self.instances_layout.clear_widgets()
        
        if self.manager and self.manager.processes:
            for instance in self.manager.processes:
                widget = InstanceWidget(instance)
                self.instances_layout.add_widget(widget)
    
    def update_interface(self, dt):
        """Actualización cada minuto"""
        if self.manager and self.manager.processes:
            active_count = 0
            for instance in self.manager.processes:
                if instance['process'].poll() is None:
                    active_count += 1
            
            if active_count == 0:
                self.status_label.text = "🔴 Todas las instancias inactivas"
                self.start_btn.disabled = False
                self.stop_btn.disabled = True
            elif active_count < len(self.manager.processes):
                self.status_label.text = f"🟡 {active_count}/{len(self.manager.processes)} instancias activas"
            else:
                self.status_label.text = f"🟢 Cluster activo - {active_count} instancias"
    
    def quick_update(self, dt):
        """Actualización rápida de estados"""
        if self.manager and self.manager.processes:
            # Actualizar widgets de instancias
            for widget in self.instances_layout.children:
                if isinstance(widget, InstanceWidget):
                    widget.update_status()
    
    def on_stop(self):
        """Cleanup al cerrar la aplicación"""
        self.running = False
        if self.manager:
            self.manager.stop_all_instances()

# Clase GoAppManager mejorada
class GoAppManager:
    def __init__(self):
        self.processes = []
        self.ports = [3001, 3002, 3003, 3004, 3005, 3006, 3007, 3008, 3009, 
                     3010, 3011, 3012, 3013, 3014, 3015, 3016, 3017, 3018, 
                     3019, 3020, 3021, 3022, 3023]
        self.seed_node = "localhost:3000"
        
    def create_data_directories(self):
        """Crear directorios de datos necesarios"""
        try:
            if not os.path.exists("data"):
                os.makedirs("data")
            
            principal_dir = "data/principal"
            if not os.path.exists(principal_dir):
                os.makedirs(principal_dir)
            
            for i in range(2, len(self.ports) + 1):
                delegate_dir = f"data/delegate-{i:02d}"
                if not os.path.exists(delegate_dir):
                    os.makedirs(delegate_dir)
                    
        except Exception as e:
            print(f"Error creando directorios: {e}")
            return False
        return True
    
    def start_instance(self, port, datadir, is_principal=False):
        """Iniciar una instancia específica"""
        try:
            cmd = ["go", "run", "main.go", f"-port={port}", f"-datadir={datadir}", 
                   "-verbose=true", f"-seed={self.seed_node}"]
            
            if os.name == 'nt':
                process = subprocess.Popen(
                    cmd,
                    stdout=subprocess.PIPE,
                    stderr=subprocess.PIPE,
                    creationflags=subprocess.CREATE_NO_WINDOW,
                    text=True,
                    bufsize=1,
                    universal_newlines=True
                )
            else:
                process = subprocess.Popen(
                    cmd,
                    stdout=subprocess.PIPE,
                    stderr=subprocess.PIPE,
                    preexec_fn=os.setsid,
                    text=True,
                    bufsize=1,
                    universal_newlines=True
                )
            
            self.processes.append({
                'process': process,
                'port': port,
                'type': 'PRINCIPAL' if is_principal else f'DELEGADO-{len(self.processes):02d}',
                'datadir': datadir
            })
            
            return True
            
        except Exception as e:
            print(f"Error iniciando instancia en puerto {port}: {e}")
            return False
    
    def start_all_instances(self):
        """Iniciar todas las instancias"""
        if not self.create_data_directories():
            return False
        
        # Iniciar nodo principal
        if not self.start_instance(self.ports[0], "data/principal", True):
            return False
        
        time.sleep(3)  # Esperar que el principal esté listo
        
        # Iniciar delegados
        for i, port in enumerate(self.ports[1:], 2):
            datadir = f"data/delegate-{i:02d}"
            if not self.start_instance(port, datadir, False):
                print(f"Error iniciando delegado en puerto {port}")
            time.sleep(0.2)  # Pequeño delay
        
        return len(self.processes) > 0
    
    def stop_all_instances(self):
        """Detener todas las instancias"""
        for instance in self.processes:
            try:
                process = instance['process']
                if process.poll() is None:
                    if os.name == 'nt':
                        process.terminate()
                    else:
                        os.killpg(os.getpgid(process.pid), signal.SIGTERM)
            except Exception:
                pass
        
        time.sleep(1)
        
        # Forzar cierre si es necesario
        for instance in self.processes:
            try:
                process = instance['process']
                if process.poll() is None:
                    if os.name == 'nt':
                        process.kill()
                    else:
                        os.killpg(os.getpgid(process.pid), signal.SIGKILL)
            except Exception:
                pass
        
        self.processes.clear()

if __name__ == "__main__":
    GoClusterMonitorApp().run()