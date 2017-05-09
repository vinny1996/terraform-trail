# provider "azurerm" {
#   subscription_id = "REPLACE-WITH-YOUR-SUBSCRIPTION-ID"
#   client_id       = "REPLACE-WITH-YOUR-CLIENT-ID"
#   client_secret   = "REPLACE-WITH-YOUR-CLIENT-SECRET"
#   tenant_id       = "REPLACE-WITH-YOUR-TENANT-ID"
# }
resource "azurerm_resource_group" "rg" {
  name     = "${var.resource_group}"
  location = "${var.location}"
}

resource "azurerm_network_interface" "nic" {
  name                = "nic"
  location            = "${var.location}"
  resource_group_name = "${var.resource_group}"

  ip_configuration {
    name                          = "ipconfig"
    subnet_id                     = "${var.existing_subnet_id}" # How do I get the subnet_id of the existing subnet? 
    private_ip_address_allocation = "Dynamic"
    public_ip_address_id          = "${azurerm_public_ip.pip.id}"
  }
}


# terraform import azurerm_virtual_network.testNetwork /subscriptions/${var.subscription_id}/resourceGroups/${var.existing_vnet_resource_group}/providers/Microsoft.Network/virtualNetworks/${var.existing_virtual_network_name}

# terraform import azurerm_virtual_network.testNetwork /subscriptions/d523ee9a-becd-48d7-a28d-44af5b6c1e30/resourceGroups/permanent/providers/Microsoft.Network/virtualNetworks/vqeeopeictwmvnet

# terraform import azurerm_subnet.testSubnet /subscriptions/d523ee9a-becd-48d7-a28d-44af5b6c1e30/resourceGroups/permanent/providers/Microsoft.Network/virtualNetworks/vqeeopeictwmvnet/subnets/vqeeopeictwmsubnet


resource "azurerm_public_ip" "pip" {
  name                         = "PublicIp"
  location                     = "${var.location}"
  resource_group_name          = "${var.resource_group}"
  public_ip_address_allocation = "Dynamic"
  domain_name_label            = "${var.hostname}"
}

resource "azurerm_storage_account" "stor" {
  name                = "bootdiagstor"
  resource_group_name = "${var.resource_group}"
  location            = "${var.location}"
  account_type        = "${var.storage_account_type}"
}

resource "azurerm_virtual_machine" "vm" {
  name                  = "${var.hostname}"
  location              = "${var.location}"
  resource_group_name   = "${var.resource_group}"
  vm_size               = "${var.vm_size}"
  network_interface_ids = ["${azurerm_network_interface.nic.id}"]

  storage_os_disk {
    name          = "${var.hostname}osdisk1"
    vhd_uri       = "${var.os_disk_vhd_uri}"
    os_type       = "${var.os_type}"
    caching       = "ReadWrite"
    create_option = "Attach"
  }

  os_profile {
    computer_name  = "${var.hostname}"
    admin_username = "${var.admin_username}"
    admin_password = "${var.admin_password}"
  }

  os_profile_linux_config {
    disable_password_authentication = false
  }

  boot_diagnostics {
    enabled     = true
    storage_uri = "${azurerm_storage_account.stor.primary_blob_endpoint}"
  }
}
