# source 09dfafa821bcc4b0
# Configuración de Nacomline. Después de un cambio: 'nacomline config apply' (los
# ajustes de la página web lo hacen al guardar). Las líneas que empiezan con '#' son
# comentarios.

[general]
# en: language of Nacomline's messages, help, menus and web page: "en", "es", "fr", "pt", "ar" (empty: this computer's)
# es: idioma de los mensajes, la ayuda, los menús y la página web de Nacomline: "en", "es", "fr", "pt", "ar" (vacío: el de esta computadora)
# fr: langue des messages, de l'aide, des menus et de la page web de Nacomline : "en", "es", "fr", "pt", "ar" (vide : celle de cet ordinateur)
# pt: idioma das mensagens, da ajuda, dos menus e da página web do Nacomline: "en", "es", "fr", "pt", "ar" (vazio: o deste computador)
# ar: لغة رسائل Nacomline ومساعدته وقوائمه وصفحة الويب: "en", "es", "fr", "pt", "ar" (فارغ: لغة هذا الحاسوب؛ الترجمة العربية آلية ولم تُراجَع بعد)
language =

[project]
# carpeta con input/ output/ weird/ errors/ ...; "." = la carpeta de este archivo
folder = .

[orca]
# instalación de ORCA (init la encuentra)
path =

[computer]
# núcleos que puede usar ORCA; 0 = todos los núcleos físicos
cores = 0
# memoria total que puede usar ORCA (p. ej. 16GiB); "auto" = la que la computadora
# puede ceder
memory = auto
# cuándo empiezan los cálculos nuevos (los que corren siempre terminan); vacío =
# siempre.
# p. ej.: mon-fri 20:00-07:00, sat-sun 00:00-24:00
schedule =
# no tomar cálculos nuevos mientras funciona con batería
pause_on_battery = true
# no tomar cálculos nuevos por encima de esta temperatura de la CPU (Celsius);
# 0 = sin límite
max_cpu_temperature = 0

[checks]
# La salida de ORCA siempre se revisa (los errores, las optimizaciones que no convergen
# y las geometrías inconsistentes van a weird/ o errors/). verify = "yes" además recalcula
# en parte algunos resultados (energía SCF, gradiente, hessiana) en esta misma
# computadora: aquí no sirve contra trampas, pero puede revelar una máquina defectuosa
# (memoria dañada, sobrecalentamiento) que cambie un resultado sin que ORCA lo note.
# Cuesta algo de tiempo de cálculo extra.
verify = no

[alerts]
# programa que se ejecuta con cada alerta (evento, mensaje), p. ej. un script de
# Telegram; vacío = ninguno
command =

[log]
# "debug", "info", "warn" o "error"
level = info
