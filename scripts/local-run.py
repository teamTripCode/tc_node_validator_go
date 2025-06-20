import subprocess
import os
import time
import argparse
import signal
import sys
import psutil
from pathlib import Path
import threading
import atexit

class ClusterManager:
    def __init__(self):
        self.processes = []
        self.child_processes = []
        self.running = False
        self.cleanup_done = False
        
        # Registrar cleanup al salir
        atexit.register(self.cleanup)
        
        # Manejar señales
        signal.signal(signal.SIGINT, self.signal_handler)
        signal.signal(signal.SIGTERM, self.signal_handler)
        
    def signal_handler(self, signum, frame):
        """Maneja señales de terminación"""
        print(f"\n🛑 Recibida señal {signum}, deteniendo cluster...")
        self.stop_cluster()
        sys.exit(0)
    
    def find_go_processes(self):
        """Encuentra todos los procesos Go relacionados con main.go"""
        go_processes = []
        try:
            for proc in psutil.process_iter(['pid', 'name', 'cmdline']):
                try:
                    cmdline = proc.info['cmdline']
                    if cmdline and any('main.go' in str(cmd) for cmd in cmdline):
                        go_processes.append(proc.info['pid'])
                except (psutil.NoSuchProcess, psutil.AccessDenied):
                    continue
        except Exception as e:
            print(f"Error buscando procesos Go: {e}")
        return go_processes
    
    def monitor_processes(self):
        """Monitorea los procesos y actualiza la lista de PIDs hijos"""
        while self.running:
            try:
                current_children = []
                for process in self.processes:
                    if process.poll() is None:  # Proceso aún activo
                        try:
                            # Agregar proceso principal
                            current_children.append(process.pid)
                            
                            # Buscar procesos hijos
                            parent = psutil.Process(process.pid)
                            for child in parent.children(recursive=True):
                                current_children.append(child.pid)
                        except (psutil.NoSuchProcess, psutil.AccessDenied):
                            continue
                
                self.child_processes = current_children
                time.sleep(1)
            except Exception:
                break
    
    def start_cluster(self, show_gui=False):
        """Inicia el cluster de nodos"""
        # Rutas relativas desde /scripts
        main_go_path = Path("../main.go")
        data_dir = Path("../data")
        
        # Verificar que main.go existe
        if not main_go_path.exists():
            print(f"❌ Error: No se encontró {main_go_path.absolute()}")
            return False
        
        # Crear directorio data si no existe
        data_dir.mkdir(exist_ok=True)
        
        # Limpiar procesos Go existentes
        print("🧹 Limpiando procesos Go existentes...")
        self.cleanup_existing_go_processes()
        
        # Puertos del 3001 al 3025
        ports = list(range(3001, 3026))
        seed = "localhost:3000"
        
        self.processes = []
        self.running = True
        
        # Iniciar monitor de procesos en hilo separado
        monitor_thread = threading.Thread(target=self.monitor_processes, daemon=True)
        monitor_thread.start()
        
        for i, port in enumerate(ports):
            # Crear directorio para cada instancia
            if port == 3001:
                instance_dir = data_dir
            else:
                delegate_number = i
                instance_dir = data_dir / f"delegate-{delegate_number}"
            
            instance_dir.mkdir(exist_ok=True)
            
            # Comando para ejecutar
            cmd = [
                "go", "run", str(main_go_path),
                f"-port={port}",
                f"-datadir={instance_dir.absolute()}",
                f"-seed={seed}"
            ]
            
            try:
                if show_gui:
                    process = self.start_gui_process(cmd, port, i)
                else:
                    process = self.start_background_process(cmd)
                
                if process:
                    self.processes.append(process)
                    gui_status = "con terminal gráfica" if show_gui else "en background"
                    print(f"✅ Nodo {i+1}/25 en puerto {port} (PID: {process.pid}) - {gui_status}")
                else:
                    print(f"❌ Error iniciando nodo en puerto {port}")
                    
            except Exception as e:
                print(f"❌ Error iniciando nodo {port}: {e}")
            
            # Pequeña pausa entre inicios
            time.sleep(0.2)
        
        print(f"\n🚀 Cluster iniciado con {len(self.processes)} nodos")
        if show_gui:
            print("📺 Nodos ejecutándose en terminales gráficas")
        else:
            print("🔇 Nodos ejecutándose en background")
        print("💡 Para detener: Ctrl+C")
        
        # Mantener el script corriendo
        try:
            while self.running:
                # Verificar si algún proceso ha terminado
                active_processes = [p for p in self.processes if p.poll() is None]
                if len(active_processes) != len(self.processes):
                    print(f"⚠️  Algunos procesos han terminado. Activos: {len(active_processes)}/{len(self.processes)}")
                    self.processes = active_processes
                
                time.sleep(1)
        except KeyboardInterrupt:
            print("\n🛑 Deteniendo cluster...")
        
        self.stop_cluster()
        return True
    
    def start_gui_process(self, cmd, port, index):
        """Inicia un proceso con interfaz gráfica"""
        if os.name == 'nt':  # Windows
            # Crear comando para nueva ventana
            window_title = f"Nodo {index+1} - Puerto {port}"
            full_cmd = [
                'cmd', '/c', 'start', 
                f'"{window_title}"',
                'cmd', '/k'
            ] + cmd
            
            return subprocess.Popen(
                full_cmd,
                shell=True,
                creationflags=subprocess.CREATE_NEW_CONSOLE
            )
        else:  # Linux/Mac
            # Lista de terminales a probar
            terminals = [
                ('gnome-terminal', ['--title', f'Nodo {index+1} - Puerto {port}', '--']),
                ('konsole', ['--title', f'Nodo {index+1} - Puerto {port}', '-e']),
                ('xterm', ['-title', f'Nodo {index+1} - Puerto {port}', '-e']),
                ('x-terminal-emulator', ['-e'])
            ]
            
            for terminal, args in terminals:
                try:
                    # Crear script de bash que mantenga la ventana abierta
                    bash_cmd = ' '.join(cmd) + '; echo "Proceso terminado. Presiona Enter para cerrar..."; read'
                    full_cmd = [terminal] + args + ['bash', '-c', bash_cmd]
                    
                    return subprocess.Popen(
                        full_cmd,
                        stdout=subprocess.DEVNULL,
                        stderr=subprocess.DEVNULL
                    )
                except FileNotFoundError:
                    continue
            
            # Si no se encuentra terminal gráfica, usar background
            print(f"⚠️  No se encontró terminal gráfica para puerto {port}, usando background")
            return self.start_background_process(cmd)
    
    def start_background_process(self, cmd):
        """Inicia un proceso en background"""
        creation_flags = 0
        if os.name == 'nt':
            creation_flags = subprocess.CREATE_NO_WINDOW
        
        return subprocess.Popen(
            cmd,
            stdout=subprocess.DEVNULL,
            stderr=subprocess.DEVNULL,
            creationflags=creation_flags
        )
    
    def cleanup_existing_go_processes(self):
        """Limpia procesos Go existentes"""
        go_pids = self.find_go_processes()
        if go_pids:
            print(f"🧹 Encontrados {len(go_pids)} procesos Go existentes, terminándolos...")
            for pid in go_pids:
                try:
                    proc = psutil.Process(pid)
                    proc.terminate()
                except (psutil.NoSuchProcess, psutil.AccessDenied):
                    continue
            
            # Esperar y forzar terminación si es necesario
            time.sleep(2)
            for pid in go_pids:
                try:
                    proc = psutil.Process(pid)
                    if proc.is_running():
                        proc.kill()
                except (psutil.NoSuchProcess, psutil.AccessDenied):
                    continue
    
    def stop_cluster(self):
        """Detiene el cluster de forma segura"""
        if self.cleanup_done:
            return
        
        self.running = False
        self.cleanup_done = True
        
        print("🛑 Deteniendo procesos del cluster...")
        
        # Obtener lista completa de PIDs a terminar
        all_pids = set()
        
        # Agregar procesos principales
        for process in self.processes:
            if process.poll() is None:
                all_pids.add(process.pid)
        
        # Agregar procesos hijos monitoreados
        all_pids.update(self.child_processes)
        
        # Buscar procesos Go adicionales
        all_pids.update(self.find_go_processes())
        
        # Terminar procesos gracefully
        terminated_pids = []
        for pid in all_pids:
            try:
                proc = psutil.Process(pid)
                proc.terminate()
                terminated_pids.append(pid)
            except (psutil.NoSuchProcess, psutil.AccessDenied):
                continue
        
        # Esperar terminación graceful
        print("⏳ Esperando terminación graceful...")
        time.sleep(3)
        
        # Forzar terminación de procesos persistentes
        killed_count = 0
        for pid in terminated_pids:
            try:
                proc = psutil.Process(pid)
                if proc.is_running():
                    proc.kill()
                    killed_count += 1
            except (psutil.NoSuchProcess, psutil.AccessDenied):
                continue
        
        # Limpiar procesos Python relacionados
        self.cleanup_python_processes()
        
        if killed_count > 0:
            print(f"🔨 Forzada terminación de {killed_count} procesos persistentes")
        
        print("✅ Cluster completamente detenido")
        
    def cleanup_python_processes(self):
        """Limpia procesos Python relacionados con el cluster"""
        try:
            current_pid = os.getpid()
            for proc in psutil.process_iter(['pid', 'name', 'cmdline']):
                try:
                    if proc.info['pid'] == current_pid:
                        continue
                    
                    cmdline = proc.info['cmdline']
                    if (cmdline and 
                        any('python' in str(cmd).lower() for cmd in cmdline) and
                        any('cluster' in str(cmd).lower() for cmd in cmdline)):
                        
                        proc.terminate()
                except (psutil.NoSuchProcess, psutil.AccessDenied):
                    continue
        except Exception:
            pass
    
    def cleanup(self):
        """Método de limpieza llamado al salir"""
        if not self.cleanup_done:
            self.stop_cluster()

def main():
    # Verificar que psutil esté disponible
    try:
        import psutil
    except ImportError:
        print("❌ Error: psutil no está instalado")
        print("💡 Instala con: pip install psutil")
        sys.exit(1)
    
    parser = argparse.ArgumentParser(description='Iniciar cluster de nodos Go')
    parser.add_argument(
        '--gui', 
        action='store_true',
        help='Mostrar cada nodo en una terminal gráfica separada'
    )
    parser.add_argument(
        '--no-gui',
        action='store_true',
        help='Ejecutar nodos en background sin ventanas (por defecto)'
    )
    
    args = parser.parse_args()
    
    # Por defecto sin GUI, a menos que se especifique --gui
    show_gui = args.gui and not args.no_gui
    
    print("🚀 Iniciando Cluster Manager...")
    if show_gui:
        print("🖥️  Modo: Terminales gráficas")
    else:
        print("🔇 Modo: Background")
    
    cluster = ClusterManager()
    cluster.start_cluster(show_gui=show_gui)

if __name__ == "__main__":
    main()